package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/r9s-ai/open-next-router/internal/onrserver"
	"github.com/r9s-ai/open-next-router/internal/version"
	"gopkg.in/yaml.v3"
)

func main() {
	var cfgPath string
	var signalCmd string
	var showVersion bool
	flag.StringVar(&cfgPath, "config", "onr.yaml", "path to config yaml")
	flag.StringVar(&cfgPath, "c", "onr.yaml", "path to config yaml (alias of --config)")
	flag.StringVar(&signalCmd, "s", "", "send signal to a running onr (supported: reload)")
	flag.BoolVar(&showVersion, "version", false, "show version information")
	flag.Parse()

	// Show version and exit
	if showVersion {
		fmt.Println(version.Get())
		return
	}

	if strings.TrimSpace(signalCmd) != "" {
		switch strings.ToLower(strings.TrimSpace(signalCmd)) {
		case "reload":
			if err := sendReloadSignal(cfgPath); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			return
		default:
			_, _ = fmt.Fprintln(os.Stderr, "unsupported -s value: "+strings.TrimSpace(signalCmd)+" (supported: reload)")
			os.Exit(2)
		}
	}

	if err := onrserver.Run(cfgPath); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func sendReloadSignal(cfgPath string) error {
	pidFile, err := pidFileFromConfig(cfgPath)
	if err != nil {
		return err
	}
	// #nosec G304 -- pid file path comes from trusted config/env.
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("read pid file %q: %w", pidFile, err)
	}
	pidStr := strings.TrimSpace(string(b))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return fmt.Errorf("invalid pid in %q: %q", pidFile, pidStr)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process pid=%d: %w", pid, err)
	}
	if err := p.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP pid=%d: %w", pid, err)
	}
	return nil
}

func pidFileFromConfig(cfgPath string) (string, error) {
	// Default must match internal/config defaults.
	const def = "/var/run/onr.pid"
	path := strings.TrimSpace(cfgPath)
	if path == "" {
		return def, nil
	}
	// #nosec G304 -- config path comes from trusted flag.
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read config %q: %w", path, err)
	}
	var partial struct {
		Server struct {
			PidFile string `yaml:"pid_file"`
		} `yaml:"server"`
	}
	if err := yaml.Unmarshal(b, &partial); err != nil {
		return "", fmt.Errorf("parse config %q: %w", path, err)
	}
	if v := strings.TrimSpace(partial.Server.PidFile); v != "" {
		return v, nil
	}
	return def, nil
}
