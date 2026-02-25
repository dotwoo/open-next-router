package cli

import (
	"strings"

	adminweb "github.com/r9s-ai/open-next-router/onr-admin/internal/web"
	"github.com/spf13/cobra"
)

type webOptions struct {
	cfgPath      string
	providersDir string
	listen       string
}

func newWebCmd() *cobra.Command {
	opts := webOptions{
		cfgPath: "onr.yaml",
		listen:  "127.0.0.1:3310",
	}
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start web UI for provider config validate/save",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWebWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.cfgPath, "config", "c", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.providersDir, "providers-dir", "", "providers dir path")
	fs.StringVar(&opts.listen, "listen", "127.0.0.1:3310", "http listen address")
	return cmd
}

func runWebWithOptions(opts webOptions) error {
	return adminweb.Run(adminweb.Options{
		ConfigPath:   strings.TrimSpace(opts.cfgPath),
		ProvidersDir: strings.TrimSpace(opts.providersDir),
		Listen:       strings.TrimSpace(opts.listen),
	})
}
