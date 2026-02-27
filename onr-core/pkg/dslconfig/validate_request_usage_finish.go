package dslconfig

import (
	"fmt"
	"strings"
)

func validateProviderRequestTransform(path, providerName string, req ProviderRequestTransform) error {
	if err := validateRequestTransform(path, providerName, "defaults.request", req.Defaults); err != nil {
		return err
	}
	for i, m := range req.Matches {
		scope := fmt.Sprintf("match[%d].request", i)
		if err := validateRequestTransform(path, providerName, scope, m.Transform); err != nil {
			return err
		}
	}
	return nil
}

func validateRequestTransform(path, providerName, scope string, t RequestTransform) error {
	mode := strings.ToLower(strings.TrimSpace(t.ReqMapMode))
	if mode == "" {
		return nil
	}
	switch mode {
	case "openai_chat_to_openai_responses":
		return nil
	case "openai_chat_to_anthropic_messages":
		return nil
	case "openai_chat_to_gemini_generate_content":
		return nil
	case "anthropic_to_openai_chat":
		return nil
	case "gemini_to_openai_chat":
		return nil
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported req_map mode %q", providerName, path, scope, t.ReqMapMode),
			scope,
			"req_map",
		)
	}
}

func validateProviderUsage(path, providerName string, usage ProviderUsage) error {
	if err := validateUsageExtractConfig(path, providerName, "defaults.metrics", usage.Defaults); err != nil {
		return err
	}
	for i, m := range usage.Matches {
		scope := fmt.Sprintf("match[%d].metrics", i)
		if err := validateUsageExtractConfig(path, providerName, scope, m.Extract); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderFinishReason(path, providerName string, finish ProviderFinishReason) error {
	if err := validateFinishReasonExtractConfig(path, providerName, "defaults.metrics", finish.Defaults); err != nil {
		return err
	}
	for i, m := range finish.Matches {
		scope := fmt.Sprintf("match[%d].metrics", i)
		if err := validateFinishReasonExtractConfig(path, providerName, scope, m.Extract); err != nil {
			return err
		}
	}
	return nil
}
func validateFinishReasonExtractConfig(path, providerName, scope string, cfg FinishReasonExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	p := strings.TrimSpace(cfg.FinishReasonPath)

	if mode == "" && p == "" {
		return nil
	}
	switch mode {
	case "", "openai", "anthropic", "gemini":
		// ok
	case usageModeCustom:
		if p == "" {
			return fmt.Errorf("provider %q in %q: %s finish_reason_extract custom requires finish_reason_path", providerName, path, scope)
		}
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: %s unsupported finish_reason_extract mode %q", providerName, path, scope, cfg.Mode),
			scope,
			"finish_reason_extract",
		)
	}
	if p != "" && !strings.HasPrefix(p, "$.") {
		return fmt.Errorf("provider %q in %q: %s finish_reason_path must start with $. ", providerName, path, scope)
	}
	return nil
}

func validateUsageExtractConfig(path, providerName, scope string, cfg UsageExtractConfig) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return nil
	}
	if mode != usageModeCustom {
		return nil
	}
	if cfg.InputTokensExpr == nil && strings.TrimSpace(cfg.InputTokensPath) == "" {
		return fmt.Errorf("provider %q in %q: %s requires input_tokens (expr) or input_tokens_path", providerName, path, scope)
	}
	if cfg.OutputTokensExpr == nil && strings.TrimSpace(cfg.OutputTokensPath) == "" {
		return fmt.Errorf("provider %q in %q: %s requires output_tokens (expr) or output_tokens_path", providerName, path, scope)
	}

	if cfg.InputTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.InputTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s input_tokens_path must start with $. ", providerName, path, scope)
	}
	if cfg.OutputTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.OutputTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s output_tokens_path must start with $. ", providerName, path, scope)
	}
	if cfg.CacheReadTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.CacheReadTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s cache_read_tokens_path must start with $. ", providerName, path, scope)
	}
	if cfg.CacheWriteTokensPath != "" && !strings.HasPrefix(strings.TrimSpace(cfg.CacheWriteTokensPath), "$.") {
		return fmt.Errorf("provider %q in %q: %s cache_write_tokens_path must start with $. ", providerName, path, scope)
	}
	return nil
}
