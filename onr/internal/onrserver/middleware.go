package onrserver

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/appnameinfer"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestid"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

type contextFieldSpec struct {
	ctxKey string
	logKey string
}

type accessLogRecord struct {
	RequestID string
	AppName   string
	LatencyMS int64
	Extras    map[string]any
}

func (r accessLogRecord) Fields() map[string]any {
	out := make(map[string]any, len(r.Extras)+3)
	if strings.TrimSpace(r.RequestID) != "" {
		out["request_id"] = r.RequestID
	}
	if strings.TrimSpace(r.AppName) != "" {
		out["appname"] = r.AppName
	}
	out["latency_ms"] = r.LatencyMS
	for k, v := range r.Extras {
		out[k] = v
	}
	return out
}

var accessLogContextFieldSpecs = []contextFieldSpec{
	{ctxKey: "onr.provider", logKey: "provider"},
	{ctxKey: "onr.provider_source", logKey: "provider_source"},
	{ctxKey: "onr.api", logKey: "api"},
	{ctxKey: "onr.stream", logKey: "stream"},
	{ctxKey: "onr.model", logKey: "model"},
	{ctxKey: "onr.usage_stage", logKey: "usage_stage"},
	{ctxKey: "onr.usage_input_tokens", logKey: "input_tokens"},
	{ctxKey: "onr.usage_output_tokens", logKey: "output_tokens"},
	{ctxKey: "onr.usage_total_tokens", logKey: "total_tokens"},
	{ctxKey: "onr.usage_cache_read_tokens", logKey: "cache_read_tokens"},
	{ctxKey: "onr.usage_cache_write_tokens", logKey: "cache_write_tokens"},
	{ctxKey: "onr.cost_total", logKey: "cost_total"},
	{ctxKey: "onr.cost_input", logKey: "cost_input"},
	{ctxKey: "onr.cost_output", logKey: "cost_output"},
	{ctxKey: "onr.cost_cache_read", logKey: "cost_cache_read"},
	{ctxKey: "onr.cost_cache_write", logKey: "cost_cache_write"},
	{ctxKey: "onr.billable_input_tokens", logKey: "billable_input_tokens"},
	{ctxKey: "onr.cost_multiplier", logKey: "cost_multiplier"},
	{ctxKey: "onr.cost_model", logKey: "cost_model"},
	{ctxKey: "onr.cost_channel", logKey: "cost_channel"},
	{ctxKey: "onr.cost_unit", logKey: "cost_unit"},
	{ctxKey: "onr.upstream_status", logKey: "upstream_status"},
	{ctxKey: "onr.finish_reason", logKey: "finish_reason"},
	{ctxKey: "onr.ttft_ms", logKey: "ttft_ms"},
	{ctxKey: "onr.tps", logKey: "tps"},
}

func requestLoggerWithColor(l *log.Logger, color bool, requestIDHeaderKey string, appnameInferEnabled bool, appnameInferUnknown string, accessFormatter *logx.AccessLogFormatter) gin.HandlerFunc {
	requestIDHeaderKey = requestid.ResolveHeaderKey(requestIDHeaderKey)
	if l == nil {
		l = log.New(os.Stdout, "", log.LstdFlags)
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)
		rec := buildAccessLogRecord(c, requestIDHeaderKey, appnameInferEnabled, appnameInferUnknown, latency)
		fields := rec.Fields()

		ts := time.Now()
		if accessFormatter != nil {
			l.Println(accessFormatter.Format(ts, status, latency, c.ClientIP(), c.Request.Method, c.Request.URL.Path, fields, color))
			return
		}
		l.Println(logx.FormatRequestLineWithColor(ts, status, latency, c.ClientIP(), c.Request.Method, c.Request.URL.Path, fields, color))
	}
}

func buildAccessLogRecord(c *gin.Context, requestIDHeaderKey string, appnameInferEnabled bool, appnameInferUnknown string, latency time.Duration) accessLogRecord {
	rec := accessLogRecord{
		RequestID: c.GetString(requestIDHeaderKey),
		AppName:   resolveAppNameForLog(c, appnameInferEnabled, appnameInferUnknown),
		LatencyMS: latency.Milliseconds(),
		Extras:    map[string]any{},
	}
	if v, ok := c.Get("onr.latency_ms"); ok {
		switch n := v.(type) {
		case int64:
			rec.LatencyMS = n
		case int:
			rec.LatencyMS = int64(n)
		default:
			rec.Extras["latency_ms"] = v
		}
	}
	copyContextFieldsBySpec(c, rec.Extras, accessLogContextFieldSpecs)
	return rec
}

func copyContextFieldsBySpec(c *gin.Context, dst map[string]any, specs []contextFieldSpec) {
	for _, s := range specs {
		if v, ok := c.Get(s.ctxKey); ok {
			dst[s.logKey] = v
		}
	}
}

func resolveAppNameForLog(c *gin.Context, inferEnabled bool, inferUnknown string) string {
	if c == nil {
		return ""
	}
	if v := strings.TrimSpace(c.GetHeader("appname")); v != "" {
		return v
	}
	if !inferEnabled {
		return ""
	}
	if v, ok := appnameinfer.Infer(c.GetHeader("User-Agent")); ok {
		return v
	}
	return strings.TrimSpace(inferUnknown)
}
