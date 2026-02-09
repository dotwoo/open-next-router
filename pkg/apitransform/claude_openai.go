package apitransform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/pkg/jsonutil"
)

const (
	claudeContentTypeToolUse = "tool_use"
	finishReasonToolCalls    = "tool_calls"
)

// MapClaudeMessagesToOpenAIChatCompletions maps Claude messages request JSON to OpenAI chat request JSON.
func MapClaudeMessagesToOpenAIChatCompletions(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "claude request")
	if err != nil {
		return nil, err
	}
	out, err := MapClaudeMessagesToOpenAIChatCompletionsObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapClaudeMessagesToOpenAIChatCompletionsObject maps Claude messages object to OpenAI chat request object.
func MapClaudeMessagesToOpenAIChatCompletionsObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	model := strings.TrimSpace(jsonutil.CoerceString(root["model"]))
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	out := apitypes.JSONObject{
		"model": model,
	}
	if s, ok := root["stream"].(bool); ok {
		out["stream"] = s
	}
	if v := jsonutil.CoerceInt(root["max_tokens"]); v > 0 {
		out["max_tokens"] = v
	}

	openAIMessages := mapClaudeMessages(root["messages"])
	openAIMessages = prependClaudeSystemMessages(root["system"], openAIMessages)

	out["messages"] = openAIMessages
	return out, nil
}

func mapClaudeMessages(rawMessages any) []any {
	messages, _ := rawMessages.([]any)
	out := make([]any, 0, len(messages)+1)
	for _, raw := range messages {
		msg, _ := raw.(map[string]any)
		if msg == nil {
			continue
		}
		items := mapOneClaudeMessage(msg)
		out = append(out, items...)
	}
	return out
}

func mapOneClaudeMessage(msg map[string]any) []any {
	role := strings.TrimSpace(jsonutil.CoerceString(msg["role"]))
	if role == "" {
		return nil
	}
	content := msg["content"]
	parts, isArray := content.([]any)
	if !isArray {
		return []any{apitypes.JSONObject{"role": role, "content": content}}
	}

	textParts := make([]string, 0, len(parts))
	toolCalls := make([]any, 0, 2)
	toolMessages := make([]any, 0, 2)
	for _, p := range parts {
		pm, _ := p.(map[string]any)
		if pm == nil {
			continue
		}
		if text, ok := claudeTextPart(pm); ok {
			textParts = append(textParts, text)
			continue
		}
		if toolCall, ok := claudeToolUsePart(pm); ok {
			toolCalls = append(toolCalls, toolCall)
			continue
		}
		if toolMsg, ok := claudeToolResultPart(pm); ok {
			toolMessages = append(toolMessages, toolMsg)
		}
	}

	item := apitypes.JSONObject{"role": role, "content": strings.Join(textParts, "\n")}
	if len(toolCalls) > 0 {
		item["tool_calls"] = toolCalls
	}
	return append([]any{item}, toolMessages...)
}

func claudeTextPart(pm map[string]any) (string, bool) {
	if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != chatContentTypeText {
		return "", false
	}
	t := strings.TrimSpace(jsonutil.CoerceString(pm["text"]))
	return t, t != ""
}

func claudeToolUsePart(pm map[string]any) (apitypes.JSONObject, bool) {
	if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != claudeContentTypeToolUse {
		return nil, false
	}
	name := strings.TrimSpace(jsonutil.CoerceString(pm["name"]))
	if name == "" {
		return nil, false
	}
	id := strings.TrimSpace(jsonutil.CoerceString(pm["id"]))
	args := "{}"
	if pm["input"] != nil {
		if b, err := json.Marshal(pm["input"]); err == nil {
			args = string(b)
		}
	}
	return apitypes.JSONObject{
		"id":   id,
		"type": chatRoleFunction,
		"function": apitypes.JSONObject{
			"name":      name,
			"arguments": args,
		},
	}, true
}

func claudeToolResultPart(pm map[string]any) (apitypes.JSONObject, bool) {
	if strings.TrimSpace(jsonutil.CoerceString(pm["type"])) != "tool_result" {
		return nil, false
	}
	callID := strings.TrimSpace(jsonutil.CoerceString(pm["tool_use_id"]))
	if callID == "" {
		return nil, false
	}
	output := jsonutil.CoerceString(pm["content"])
	if output == "" && pm["content"] != nil {
		if b, err := json.Marshal(pm["content"]); err == nil {
			output = string(b)
		}
	}
	return apitypes.JSONObject{
		"role":         "tool",
		"tool_call_id": callID,
		"content":      output,
	}, true
}

func prependClaudeSystemMessages(rawSystem any, openAIMessages []any) []any {
	switch v := rawSystem.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return append([]any{apitypes.JSONObject{"role": "system", "content": v}}, openAIMessages...)
		}
	case []any:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			pm, _ := p.(map[string]any)
			if pm == nil {
				continue
			}
			if t, ok := claudeTextPart(pm); ok {
				parts = append(parts, t)
			}
		}
		if len(parts) > 0 {
			return append([]any{apitypes.JSONObject{"role": "system", "content": strings.Join(parts, "\n")}}, openAIMessages...)
		}
	}
	return openAIMessages
}

// MapOpenAIChatCompletionsToClaudeMessagesResponse maps OpenAI chat response JSON to Claude response JSON.
func MapOpenAIChatCompletionsToClaudeMessagesResponse(body []byte) ([]byte, error) {
	root, err := apitypes.ParseJSONObject(body, "openai response")
	if err != nil {
		return nil, err
	}
	out, err := MapOpenAIChatCompletionsToClaudeMessagesResponseObject(root)
	if err != nil {
		return nil, err
	}
	return out.Marshal()
}

// MapOpenAIChatCompletionsToClaudeMessagesResponseObject maps OpenAI chat response object to Claude response object.
func MapOpenAIChatCompletionsToClaudeMessagesResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	choices, _ := root["choices"].([]any)
	if len(choices) == 0 {
		return nil, fmt.Errorf("choices is required")
	}
	choice0, _ := choices[0].(map[string]any)
	if choice0 == nil {
		return nil, fmt.Errorf("invalid choices[0]")
	}
	msg, _ := choice0["message"].(map[string]any)
	if msg == nil {
		return nil, fmt.Errorf("invalid choices[0].message")
	}

	content := make([]any, 0, 2)
	toolCalls, _ := msg["tool_calls"].([]any)
	if len(toolCalls) > 0 {
		for _, raw := range toolCalls {
			tc, _ := raw.(map[string]any)
			if tc == nil {
				continue
			}
			fn, _ := tc["function"].(map[string]any)
			name := strings.TrimSpace(jsonutil.CoerceString(fn["name"]))
			if name == "" {
				continue
			}
			input := apitypes.JSONObject{}
			if args := strings.TrimSpace(jsonutil.CoerceString(fn["arguments"])); args != "" {
				var v any
				if err := json.Unmarshal([]byte(args), &v); err == nil {
					if m, ok := v.(map[string]any); ok && m != nil {
						input = m
					} else {
						input["arguments"] = args
					}
				} else {
					input["arguments"] = args
				}
			}
			content = append(content, apitypes.JSONObject{
				"type":  claudeContentTypeToolUse,
				"id":    jsonutil.CoerceString(tc["id"]),
				"name":  name,
				"input": input,
			})
		}
	} else {
		text := jsonutil.CoerceString(msg["content"])
		content = append(content, apitypes.JSONObject{"type": chatContentTypeText, "text": text})
	}

	stopReason := "end_turn"
	switch strings.TrimSpace(jsonutil.CoerceString(choice0["finish_reason"])) {
	case finishReasonLength:
		stopReason = "max_tokens"
	case finishReasonToolCalls:
		stopReason = claudeContentTypeToolUse
	}

	usage := apitypes.JSONObject{}
	if um, _ := root["usage"].(map[string]any); um != nil {
		usage["input_tokens"] = jsonutil.GetIntByPath(um, "$.prompt_tokens")
		if usage["input_tokens"] == 0 {
			usage["input_tokens"] = jsonutil.GetIntByPath(um, "$.input_tokens")
		}
		usage["output_tokens"] = jsonutil.GetIntByPath(um, "$.completion_tokens")
		if usage["output_tokens"] == 0 {
			usage["output_tokens"] = jsonutil.GetIntByPath(um, "$.output_tokens")
		}
	}

	out := apitypes.JSONObject{
		"id":          jsonutil.CoerceString(root["id"]),
		"type":        "message",
		"role":        "assistant",
		"model":       jsonutil.CoerceString(root["model"]),
		"content":     content,
		"stop_reason": stopReason,
	}
	if len(usage) > 0 {
		out["usage"] = usage
	}
	return out, nil
}
