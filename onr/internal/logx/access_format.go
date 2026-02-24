package logx

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

type formatPart struct {
	literal string
	varName string
}

type AccessLogFormatter struct {
	parts []formatPart
}

var allowedAccessLogVars = map[string]struct{}{
	"time_local":            {},
	"status":                {},
	"latency":               {},
	"latency_ms":            {},
	"client_ip":             {},
	"method":                {},
	"path":                  {},
	"request_id":            {},
	"appname":               {},
	"provider":              {},
	"provider_source":       {},
	"api":                   {},
	"stream":                {},
	"model":                 {},
	"usage_stage":           {},
	"input_tokens":          {},
	"output_tokens":         {},
	"total_tokens":          {},
	"cache_read_tokens":     {},
	"cache_write_tokens":    {},
	"cost_total":            {},
	"cost_input":            {},
	"cost_output":           {},
	"cost_cache_read":       {},
	"cost_cache_write":      {},
	"billable_input_tokens": {},
	"cost_multiplier":       {},
	"cost_model":            {},
	"cost_channel":          {},
	"cost_unit":             {},
	"upstream_status":       {},
	"finish_reason":         {},
	"ttft_ms":               {},
	"tps":                   {},
}

func CompileAccessLogFormat(format string) (*AccessLogFormatter, error) {
	s := strings.TrimSpace(format)
	if s == "" {
		return nil, nil
	}
	parts := make([]formatPart, 0, 8)
	var lit strings.Builder

	flushLiteral := func() {
		if lit.Len() == 0 {
			return
		}
		parts = append(parts, formatPart{literal: lit.String()})
		lit.Reset()
	}

	for i := 0; i < len(format); i++ {
		ch := format[i]
		if ch != '$' {
			lit.WriteByte(ch)
			continue
		}
		if i+1 < len(format) && format[i+1] == '$' {
			lit.WriteByte('$')
			i++
			continue
		}
		flushLiteral()
		j := i + 1
		for j < len(format) {
			r := rune(format[j])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				break
			}
			j++
		}
		if j == i+1 {
			return nil, fmt.Errorf("invalid access_log_format: missing variable name after '$' at pos %d", i)
		}
		name := format[i+1 : j]
		if _, ok := allowedAccessLogVars[name]; !ok {
			return nil, fmt.Errorf("invalid access_log_format: unknown variable $%s", name)
		}
		parts = append(parts, formatPart{varName: name})
		i = j - 1
	}
	flushLiteral()
	return &AccessLogFormatter{parts: parts}, nil
}

func (f *AccessLogFormatter) Format(
	ts time.Time,
	status int,
	latency time.Duration,
	clientIP string,
	method string,
	path string,
	fields map[string]any,
	color bool,
) string {
	if f == nil || len(f.parts) == 0 {
		return ""
	}
	vars := map[string]string{
		"time_local": ts.Format("2006/01/02 - 15:04:05"),
		"status":     ColorizeStatusWith(status, color),
		"latency":    latency.String(),
		"latency_ms": fmt.Sprintf("%d", latency.Milliseconds()),
		"client_ip":  strings.TrimSpace(clientIP),
		"method":     strings.TrimSpace(method),
		"path":       path,
	}
	for k, v := range fields {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" || s == "<nil>" {
			continue
		}
		vars[k] = s
	}

	var b strings.Builder
	for _, p := range f.parts {
		if p.literal != "" {
			b.WriteString(p.literal)
			continue
		}
		v := strings.TrimSpace(vars[p.varName])
		if v == "" {
			b.WriteByte('-')
			continue
		}
		b.WriteString(v)
	}
	return b.String()
}

func AccessLogAllowedVars() []string {
	keys := make([]string, 0, len(allowedAccessLogVars))
	for k := range allowedAccessLogVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
