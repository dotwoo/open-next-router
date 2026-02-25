package onrserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/config"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/models"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
)

type providersReloadResult struct {
	LoadResult       dslconfig.LoadResult
	ChangedProviders []string
}

func Run(cfgPath string) error {
	startedAt := time.Now().Unix()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	accessLogger, accessClose, accessColor, err := openAccessLogger(cfg)
	if err != nil {
		return fmt.Errorf("init access log: %w", err)
	}
	if accessClose != nil {
		defer func() { _ = accessClose.Close() }()
	}

	pidCleanup, err := writePIDFile(cfg)
	if err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	if pidCleanup != nil {
		defer func() { _ = pidCleanup.Close() }()
	}

	reg := dslconfig.NewRegistry()
	loadRes, err := reg.ReloadFromDir(cfg.Providers.Dir)
	if err != nil {
		return fmt.Errorf("load providers dir %q: %w", cfg.Providers.Dir, err)
	}
	logSkippedProviders(cfg.Providers.Dir, loadRes.SkippedFiles, false)

	keys, err := keystore.Load(cfg.Keys.File)
	if err != nil {
		return fmt.Errorf("load keys file %q: %w", cfg.Keys.File, err)
	}
	mr, err := models.Load(cfg.Models.File)
	if err != nil {
		return fmt.Errorf("load models file %q: %w", cfg.Models.File, err)
	}

	readTimeout := time.Duration(cfg.Server.ReadTimeoutMs) * time.Millisecond
	writeTimeout := time.Duration(cfg.Server.WriteTimeoutMs) * time.Millisecond

	httpClient := &http.Client{
		Timeout: writeTimeout,
	}

	pclient := &proxy.Client{
		HTTP:                     httpClient,
		ReadTimeout:              readTimeout,
		WriteTimeout:             writeTimeout,
		Registry:                 reg,
		UsageEst:                 &cfg.UsageEstimation,
		ProxyByProvider:          cfg.UpstreamProxies.ByProvider,
		OAuthTokenPersistEnabled: cfg.OAuth.TokenPersist.Enabled,
		OAuthTokenPersistDir:     cfg.OAuth.TokenPersist.Dir,
	}
	pricingResolver, err := pricing.LoadResolver(cfg.Pricing.File, cfg.Pricing.OverridesFile)
	if err != nil {
		return fmt.Errorf("load pricing files failed: %w", err)
	}
	pclient.SetPricingResolver(pricingResolver)
	pclient.SetPricingEnabled(cfg.Pricing.Enabled)

	st := &state{
		keys:        keys,
		modelRouter: mr,
	}
	st.SetStartedAtUnix(startedAt)

	reloadMu := &sync.Mutex{}
	installReloadSignalHandler(cfg, st, reg, pclient, reloadMu)
	autoReloadClose, err := installProvidersAutoReload(cfg, reg, reloadMu)
	if err != nil {
		return fmt.Errorf("init providers auto reload: %w", err)
	}
	if autoReloadClose != nil {
		defer func() { _ = autoReloadClose.Close() }()
	}

	accessFormat, err := logx.ResolveAccessLogFormat(cfg.Logging.AccessLogFormat, cfg.Logging.AccessLogFormatPreset)
	if err != nil {
		return fmt.Errorf("resolve access log format: %w", err)
	}
	accessFormatter, err := logx.CompileAccessLogFormat(accessFormat)
	if err != nil {
		return fmt.Errorf("compile access_log_format: %w", err)
	}
	engine := NewRouter(cfg, st, reg, pclient, accessLogger, accessColor, "X-Onr-Request-Id", accessFormatter)

	log.Printf("open-next-router listening on %s", cfg.Server.Listen)
	if err := engine.Run(cfg.Server.Listen); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func openAccessLogger(cfg *config.Config) (*log.Logger, io.Closer, bool, error) {
	if cfg == nil || !cfg.Logging.AccessLog {
		return nil, nil, false, nil
	}

	path := strings.TrimSpace(cfg.Logging.AccessLogPath)
	if path == "" {
		// default: stdout (same as current behavior)
		return log.New(os.Stdout, "", log.LstdFlags), nil, true, nil
	}

	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, nil, false, err
		}
	}
	// #nosec G304 -- access_log_path comes from trusted config/env.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, false, err
	}
	return log.New(f, "", log.LstdFlags), f, false, nil
}

type closerFunc func() error

func (c closerFunc) Close() error { return c() }

func writePIDFile(cfg *config.Config) (io.Closer, error) {
	if cfg == nil {
		return nil, nil
	}
	path := strings.TrimSpace(cfg.Server.PidFile)
	if path == "" {
		return nil, nil
	}
	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}

	tmp := path + ".tmp"
	pid := strconv.Itoa(os.Getpid()) + "\n"
	// #nosec G304 -- pid_file comes from trusted config/env.
	if err := os.WriteFile(tmp, []byte(pid), 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	return closerFunc(func() error { return os.Remove(path) }), nil
}

func installReloadSignalHandler(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client, mu *sync.Mutex) {
	if cfg == nil || st == nil || reg == nil || mu == nil {
		return
	}
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			mu.Lock()
			providersRes, err := reloadRuntime(cfg, st, reg, pclient)
			mu.Unlock()
			if err != nil {
				log.Printf("reload failed (signal): %v", err)
				continue
			}
			log.Printf(
				"reload ok (signal): providers_dir=%q changed_providers=%s keys_file=%q models_file=%q pricing_file=%q pricing_overrides_file=%q",
				cfg.Providers.Dir,
				providerNamesForLog(providersRes.ChangedProviders),
				cfg.Keys.File,
				cfg.Models.File,
				cfg.Pricing.File,
				cfg.Pricing.OverridesFile,
			)
		}
	}()
}

func reloadProvidersRuntime(cfg *config.Config, reg *dslconfig.Registry) (providersReloadResult, error) {
	if cfg == nil || reg == nil {
		return providersReloadResult{}, errors.New("reload providers: nil cfg/registry")
	}
	before := snapshotProviderFingerprints(reg)
	loadRes, err := reg.ReloadFromDir(cfg.Providers.Dir)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload providers dir %q: %w", cfg.Providers.Dir, err)
	}
	logSkippedProviders(cfg.Providers.Dir, loadRes.SkippedFiles, true)
	after := snapshotProviderFingerprints(reg)
	return providersReloadResult{
		LoadResult:       loadRes,
		ChangedProviders: diffChangedProviderNames(before, after),
	}, nil
}

func reloadRuntime(cfg *config.Config, st *state, reg *dslconfig.Registry, pclient *proxy.Client) (providersReloadResult, error) {
	if cfg == nil || st == nil || reg == nil || pclient == nil {
		return providersReloadResult{}, errors.New("reload: nil cfg/state/registry/pclient")
	}
	providersRes, err := reloadProvidersRuntime(cfg, reg)
	if err != nil {
		return providersReloadResult{}, err
	}
	ks, err := keystore.Load(cfg.Keys.File)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload keys file %q: %w", cfg.Keys.File, err)
	}
	mr, err := models.Load(cfg.Models.File)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload models file %q: %w", cfg.Models.File, err)
	}
	pricingResolver, err := pricing.LoadResolver(cfg.Pricing.File, cfg.Pricing.OverridesFile)
	if err != nil {
		return providersReloadResult{}, fmt.Errorf("reload pricing files failed: %w", err)
	}
	st.SetKeys(ks)
	st.SetModelRouter(mr)
	pclient.SetPricingResolver(pricingResolver)
	pclient.SetPricingEnabled(cfg.Pricing.Enabled)
	return providersRes, nil
}

func logSkippedProviders(providersDir string, skipped []string, reloading bool) {
	if len(skipped) == 0 {
		return
	}
	phase := "load"
	if reloading {
		phase = "reload"
	}
	warn := "WARNING"
	if logx.ColorEnabled() {
		warn = "\x1b[1;33mWARNING\x1b[0m"
	}
	log.Printf("[ONR] %s [providers/%s] dir=%q skipped_invalid_files=%s", warn, phase, providersDir, strings.Join(skipped, ", "))
}

func providerNamesForLog(names []string) string {
	if len(names) == 0 {
		return "<none>"
	}
	return strings.Join(names, ",")
}

func snapshotProviderFingerprints(reg *dslconfig.Registry) map[string]string {
	if reg == nil {
		return map[string]string{}
	}
	names := reg.ListProviderNames()
	out := make(map[string]string, len(names))
	for _, name := range names {
		pf, ok := reg.GetProvider(name)
		if !ok {
			continue
		}
		out[name] = providerFingerprint(pf)
	}
	return out
}

func providerFingerprint(pf dslconfig.ProviderFile) string {
	return strings.TrimSpace(pf.Path) + "\x00" + pf.Content
}

func diffChangedProviderNames(before map[string]string, after map[string]string) []string {
	changed := make([]string, 0)
	for name, prev := range before {
		next, ok := after[name]
		if !ok || next != prev {
			changed = append(changed, name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}
