package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
)

func Run(cfgPath string, in io.Reader, out io.Writer) error {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	dumpsDir := "./dumps"
	if cfg != nil && strings.TrimSpace(cfg.TrafficDump.Dir) != "" {
		dumpsDir = strings.TrimSpace(cfg.TrafficDump.Dir)
	}

	p, err := newDumpViewerProgram(dumpsDir, in, out)
	if err != nil {
		return err
	}
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui run failed: %w", err)
	}
	return nil
}
