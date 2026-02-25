package logx

import (
	"strings"
	"testing"
	"time"
)

func TestCompileAccessLogFormat(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		f, err := CompileAccessLogFormat("   ")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if f != nil {
			t.Fatalf("expected nil formatter")
		}
	})

	t.Run("unknown variable fails", func(t *testing.T) {
		_, err := CompileAccessLogFormat("$unknown")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("render with missing var uses dash", func(t *testing.T) {
		f, err := CompileAccessLogFormat("$method $path $appname")
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		out := f.Format(time.Unix(0, 0), 200, 1500*time.Millisecond, "127.0.0.1", "GET", "/v1/models", nil, false)
		if out != "GET /v1/models -" {
			t.Fatalf("unexpected out: %q", out)
		}
	})

	t.Run("dollar escape", func(t *testing.T) {
		f, err := CompileAccessLogFormat("$$ $status")
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		out := f.Format(time.Unix(0, 0), 200, time.Second, "", "", "", nil, false)
		if !strings.HasPrefix(out, "$ 200") {
			t.Fatalf("unexpected out: %q", out)
		}
	})
}

func TestResolveAccessLogFormat(t *testing.T) {
	t.Run("custom format takes precedence over preset", func(t *testing.T) {
		got, err := ResolveAccessLogFormat("$method $path", "onr_minimal")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "$method $path" {
			t.Fatalf("unexpected format: %q", got)
		}
	})

	t.Run("preset resolves when custom format empty", func(t *testing.T) {
		got, err := ResolveAccessLogFormat("  ", "onr_minimal")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("expected preset format")
		}
		if !strings.Contains(got, "$request_id") {
			t.Fatalf("expected preset format to include request id, got=%q", got)
		}
	})

	t.Run("unknown preset fails", func(t *testing.T) {
		_, err := ResolveAccessLogFormat("", "not-exist")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("empty format and preset returns empty", func(t *testing.T) {
		got, err := ResolveAccessLogFormat("", "")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "" {
			t.Fatalf("expected empty format, got=%q", got)
		}
	})
}
