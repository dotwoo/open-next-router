package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestProviderUsageSelect_MergeMatch(t *testing.T) {
	streamTrue := true
	p := ProviderUsage{
		Defaults: UsageExtractConfig{
			Mode:             usageModeOpenAI,
			InputTokensPath:  "$.usage.input",
			OutputTokensPath: "$.usage.output",
		},
		Matches: []MatchUsage{
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Extract: UsageExtractConfig{
					Mode:             usageModeCustom,
					OutputTokensPath: "$.x.out",
				},
			},
		},
	}

	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: true})
	if !ok {
		t.Fatalf("expected usage config selected")
	}
	if cfg.Mode != usageModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, usageModeCustom)
	}
	if cfg.InputTokensPath != "$.usage.input" {
		t.Fatalf("input path not merged from defaults: %q", cfg.InputTokensPath)
	}
	if cfg.OutputTokensPath != "$.x.out" {
		t.Fatalf("output path not overridden by match: %q", cfg.OutputTokensPath)
	}
}

func TestProviderFinishReasonSelect_MergeAndEmpty(t *testing.T) {
	p := ProviderFinishReason{
		Defaults: FinishReasonExtractConfig{Mode: "openai", FinishReasonPath: "$.a"},
		Matches: []MatchFinishReason{
			{
				API:    "chat.completions",
				Stream: nil,
				Extract: FinishReasonExtractConfig{
					Mode: "custom",
				},
			},
		},
	}
	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("expected finish_reason config selected")
	}
	if cfg.Mode != "custom" {
		t.Fatalf("mode=%q want=custom", cfg.Mode)
	}
	if cfg.FinishReasonPath != "$.a" {
		t.Fatalf("path should keep default when not overridden, got %q", cfg.FinishReasonPath)
	}

	if _, ok := (ProviderFinishReason{}).Select(&dslmeta.Meta{API: "chat.completions"}); ok {
		t.Fatalf("expected empty config not selected")
	}
}

func TestProviderResponseSelect_MergeDirective(t *testing.T) {
	streamFalse := false
	p := ProviderResponse{
		Defaults: ResponseDirective{
			Op:   "resp_map",
			Mode: "openai_responses_to_openai_chat",
			JSONOps: []JSONOp{
				{Op: "json_set", Path: "$.a", ValueExpr: "\"1\""},
			},
		},
		Matches: []MatchResponse{
			{
				API:    "chat.completions",
				Stream: &streamFalse,
				Response: ResponseDirective{
					JSONOps: []JSONOp{
						{Op: "json_del", Path: "$.b"},
					},
					SSEJSONDelIf: []SSEJSONDelIfRule{
						{CondPath: "$.type", Equals: "x", DelPath: "$.c"},
					},
				},
			},
		},
	}
	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: false})
	if !ok {
		t.Fatalf("expected response directive selected")
	}
	if cfg.Op != "resp_map" {
		t.Fatalf("unexpected op: %q", cfg.Op)
	}
	if len(cfg.JSONOps) != 2 {
		t.Fatalf("expected merged json ops len=2, got %d", len(cfg.JSONOps))
	}
	if len(cfg.SSEJSONDelIf) != 1 {
		t.Fatalf("expected merged sse rules len=1, got %d", len(cfg.SSEJSONDelIf))
	}
}

func TestProviderBalanceSelect_MergeMatch(t *testing.T) {
	streamTrue := true
	p := ProviderBalance{
		Defaults: BalanceQueryConfig{
			Mode:        balanceModeOpenAI,
			Method:      "GET",
			BalancePath: "$.default.balance",
		},
		Matches: []MatchBalance{
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Query: BalanceQueryConfig{
					Mode: balanceModeCustom,
					Path: "/v1/billing",
				},
			},
		},
	}
	cfg, ok := p.Select(&dslmeta.Meta{API: "chat.completions", IsStream: true})
	if !ok {
		t.Fatalf("expected balance config selected")
	}
	if cfg.Mode != balanceModeCustom {
		t.Fatalf("mode=%q want=%q", cfg.Mode, balanceModeCustom)
	}
	if cfg.BalancePath != "$.default.balance" {
		t.Fatalf("balance path should keep default, got %q", cfg.BalancePath)
	}
	if cfg.Path != "/v1/billing" {
		t.Fatalf("path not overridden by match, got %q", cfg.Path)
	}
}

func TestProviderRoutingHasMatchHelpers(t *testing.T) {
	streamTrue := true
	p := ProviderRouting{
		Matches: []RoutingMatch{
			{API: "chat.completions", Stream: &streamTrue},
		},
	}
	if !p.HasMatchAPI("chat.completions") {
		t.Fatalf("HasMatchAPI should be true")
	}
	if p.HasMatchAPI("embeddings") {
		t.Fatalf("HasMatchAPI should be false")
	}
	if !p.HasMatch(&dslmeta.Meta{API: "chat.completions", IsStream: true}) {
		t.Fatalf("HasMatch should be true for stream=true")
	}
	if p.HasMatch(&dslmeta.Meta{API: "chat.completions", IsStream: false}) {
		t.Fatalf("HasMatch should be false for stream mismatch")
	}
	if p.HasMatch(nil) {
		t.Fatalf("HasMatch should be false for nil meta")
	}
}
