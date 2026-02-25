package appnameinfer

import "strings"

type rule struct {
	app      string
	keywords []string
}

var defaultRules = []rule{
	{app: "claude-code", keywords: []string{"claude-code", "anthropic-claude-code"}},
	{app: "kilo-code", keywords: []string{"kilo-code"}},
	{app: "cursor", keywords: []string{"cursor"}},
	{app: "windsurf", keywords: []string{"windsurf", "codeium"}},
	{app: "cline", keywords: []string{"cline"}},
	{app: "roo-code", keywords: []string{"roo-code", "roo/"}},
	{app: "aider", keywords: []string{"aider"}},
	{app: "continue", keywords: []string{"continue"}},
	{app: "openai-sdk", keywords: []string{"openai-python", "openai-node", "openai-go"}},
	{app: "anthropic-sdk", keywords: []string{"anthropic-python", "anthropic-sdk"}},
}

// Infer returns a normalized app name inferred from a User-Agent string.
// The second return value indicates whether inference succeeded.
func Infer(userAgent string) (string, bool) {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return "", false
	}
	for _, r := range defaultRules {
		for _, kw := range r.keywords {
			if strings.Contains(ua, kw) {
				return r.app, true
			}
		}
	}
	return "", false
}
