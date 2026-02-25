package dslconfig

import (
	"strings"
	"testing"
)

func TestModesByDirective(t *testing.T) {
	got := ModesByDirective("req_map")
	if len(got) == 0 {
		t.Fatalf("expected req_map modes, got none")
	}
}

func TestDirectivesByBlock(t *testing.T) {
	got := DirectivesByBlock("auth")
	if len(got) == 0 {
		t.Fatalf("expected auth directives, got none")
	}
	found := false
	for _, d := range got {
		if d == "oauth_mode" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected oauth_mode in auth block directives")
	}
}

func TestDirectiveHover(t *testing.T) {
	hover, ok := DirectiveHover("response")
	if !ok || hover == "" {
		t.Fatalf("expected hover for response")
	}
}

func TestDirectiveHoverForModelsMode(t *testing.T) {
	hover, ok := DirectiveHover("models_mode")
	if !ok || hover == "" {
		t.Fatalf("expected hover for models_mode")
	}
}

func TestDirectiveHoverInBlock_PrefersExactBlock(t *testing.T) {
	hover, ok := DirectiveHoverInBlock("set_header", "balance")
	if !ok || hover == "" {
		t.Fatalf("expected hover for set_header in balance block")
	}
	if !contains(hover, "balance query request") {
		t.Fatalf("expected balance-specific hover, got: %q", hover)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
