package appnameinfer

import "testing"

func TestInfer(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
		ok   bool
	}{
		{name: "claude_code", ua: "claude-code/1.0", want: "claude-code", ok: true},
		{name: "kilo_code", ua: "kilo-code/0.9", want: "kilo-code", ok: true},
		{name: "cursor", ua: "Cursor/2.0", want: "cursor", ok: true},
		{name: "windsurf", ua: "Windsurf", want: "windsurf", ok: true},
		{name: "codeium", ua: "codeium-plugin", want: "windsurf", ok: true},
		{name: "cline", ua: "cline", want: "cline", ok: true},
		{name: "roo_code", ua: "roo-code/1", want: "roo-code", ok: true},
		{name: "aider", ua: "aider/0.1", want: "aider", ok: true},
		{name: "continue", ua: "continue", want: "continue", ok: true},
		{name: "openai_sdk", ua: "openai-python/1.2.3", want: "openai-sdk", ok: true},
		{name: "anthropic_sdk", ua: "anthropic-sdk-go/0.1", want: "anthropic-sdk", ok: true},
		{name: "unknown", ua: "Mozilla/5.0", want: "", ok: false},
		{name: "empty", ua: " ", want: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Infer(tt.ua)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("Infer(%q) = (%q, %v), want (%q, %v)", tt.ua, got, ok, tt.want, tt.ok)
			}
		})
	}
}
