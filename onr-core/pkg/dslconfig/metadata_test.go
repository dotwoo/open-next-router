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

func TestDirectiveMetadataList_ReturnsCopy(t *testing.T) {
	meta := DirectiveMetadataList()
	if len(meta) == 0 {
		t.Fatalf("expected metadata entries")
	}
	meta[0].Name = "mutated"
	meta[0].Modes = []string{"x"}
	meta[0].Args = []DirectiveArg{{Name: "x", Kind: "enum", Enum: []string{"A"}}}

	meta2 := DirectiveMetadataList()
	if len(meta2) == 0 {
		t.Fatalf("expected metadata entries in second read")
	}
	if meta2[0].Name == "mutated" {
		t.Fatalf("metadata list should return independent copy")
	}
}

func TestDirectiveArgEnumValuesInBlock(t *testing.T) {
	got := DirectiveArgEnumValuesInBlock("balance_unit", "balance", 0)
	if len(got) == 0 {
		t.Fatalf("expected enum values for balance_unit")
	}
	foundUSD := false
	for _, v := range got {
		if v == "USD" {
			foundUSD = true
			break
		}
	}
	if !foundUSD {
		t.Fatalf("expected USD in balance_unit enum values, got: %v", got)
	}

	got = DirectiveArgEnumValuesInBlock("method", "models", 0)
	if len(got) == 0 {
		t.Fatalf("expected enum values for method in models")
	}
}

func TestMetadata_ModeOptionsConsistency(t *testing.T) {
	oauthModes := make([]string, 0, len(oauthBuiltinTemplates))
	for k := range oauthBuiltinTemplates {
		oauthModes = append(oauthModes, k)
	}
	assertSetEqual(t, "oauth_mode", ModesByDirective("oauth_mode"), oauthModes)
	assertSetEqual(t, "balance_mode", ModesByDirective("balance_mode"), []string{balanceModeOpenAI, balanceModeCustom})
	assertSetEqual(t, "models_mode", ModesByDirective("models_mode"), []string{modelsModeOpenAI, modelsModeGemini, modelsModeCustom})
	assertSetEqual(t, "usage_extract", ModesByDirective("usage_extract"), []string{usageModeOpenAI, usageModeAnthropic, usageModeGemini, usageModeCustom})
	assertSetEqual(t, "finish_reason_extract", ModesByDirective("finish_reason_extract"), []string{usageModeOpenAI, usageModeAnthropic, usageModeGemini, usageModeCustom})

	errorModes := make([]string, 0, len(supportedErrorMapModes))
	for k := range supportedErrorMapModes {
		errorModes = append(errorModes, k)
	}
	assertSetEqual(t, "error_map", ModesByDirective("error_map"), errorModes)
}

func TestMetadata_EnumArgOptionsConsistency(t *testing.T) {
	assertSetEqual(t, "oauth_method.auth", DirectiveArgEnumValuesInBlock("oauth_method", "auth", 0), []string{"GET", "POST"})
	assertSetEqual(t, "oauth_content_type.auth", DirectiveArgEnumValuesInBlock("oauth_content_type", "auth", 0), []string{oauthContentTypeForm, oauthContentTypeJSON})
	assertSetEqual(t, "method.balance", DirectiveArgEnumValuesInBlock("method", "balance", 0), []string{"GET", "POST"})
	assertSetEqual(t, "method.models", DirectiveArgEnumValuesInBlock("method", "models", 0), []string{"GET", "POST"})
	assertSetEqual(t, "balance_unit.balance", DirectiveArgEnumValuesInBlock("balance_unit", "balance", 0), []string{"USD", "CNY"})
}

func assertSetEqual(t *testing.T, name string, got, want []string) {
	t.Helper()
	gotSet := make(map[string]struct{}, len(got))
	for _, v := range got {
		gotSet[strings.TrimSpace(v)] = struct{}{}
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, v := range want {
		wantSet[strings.TrimSpace(v)] = struct{}{}
	}
	if len(gotSet) != len(wantSet) {
		t.Fatalf("%s size mismatch: got=%v want=%v", name, got, want)
	}
	for v := range wantSet {
		if _, ok := gotSet[v]; !ok {
			t.Fatalf("%s missing value %q, got=%v want=%v", name, v, got, want)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
