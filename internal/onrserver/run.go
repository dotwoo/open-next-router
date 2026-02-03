package onrserver

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/internal/config"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/internal/proxy"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
)

func Run(cfgPath string) error {
	startedAt := time.Now().Unix()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	accessLogger, accessClose, err := openAccessLogger(cfg)
	if err != nil {
		return fmt.Errorf("init access log: %w", err)
	}
	if accessClose != nil {
		defer func() { _ = accessClose.Close() }()
	}

	reg := dslconfig.NewRegistry()
	if _, err := reg.ReloadFromDir(cfg.Providers.Dir); err != nil {
		return fmt.Errorf("load providers dir %q: %w", cfg.Providers.Dir, err)
	}

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
		HTTP:            httpClient,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		Registry:        reg,
		UsageEst:        &cfg.UsageEstimation,
		ProxyByProvider: cfg.UpstreamProxies.ByProvider,
	}

	st := &state{
		keys:        keys,
		modelRouter: mr,
	}
	st.SetStartedAtUnix(startedAt)

	engine := NewRouter(cfg, st, reg, pclient, accessLogger)

	log.Printf("open-next-router listening on %s", cfg.Server.Listen)
	if err := engine.Run(cfg.Server.Listen); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

type nopCloser struct{ io.Writer }

func (n nopCloser) Close() error { return nil }

func openAccessLogger(cfg *config.Config) (*log.Logger, io.Closer, error) {
	if cfg == nil || !cfg.Logging.AccessLog {
		return nil, nil, nil
	}

	path := strings.TrimSpace(cfg.Logging.AccessLogPath)
	if path == "" {
		// default: stdout (same as current behavior)
		return log.New(os.Stdout, "", log.LstdFlags), nopCloser{os.Stdout}, nil
	}

	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, nil, err
		}
	}
	// #nosec G304 -- access_log_path comes from trusted config/env.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	return log.New(f, "", log.LstdFlags), f, nil
}
