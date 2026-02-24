package onrserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr/internal/proxy"
)

func TestSetProxyResultContext_InputTokensPassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("status 200 keeps input tokens", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		setProxyResultContext(c, &proxy.Result{
			Status: http.StatusOK,
			Usage: map[string]any{
				"input_tokens":  12,
				"output_tokens": 34,
			},
		})

		v, ok := c.Get("onr.usage_input_tokens")
		if !ok || v != 12 {
			t.Fatalf("expected input_tokens=12, got ok=%v value=%v", ok, v)
		}
		v, ok = c.Get("onr.usage_output_tokens")
		if !ok || v != 34 {
			t.Fatalf("expected output_tokens=34, got ok=%v value=%v", ok, v)
		}
	})

	t.Run("non-200 keeps input tokens when result already has usage", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		setProxyResultContext(c, &proxy.Result{
			Status: http.StatusBadGateway,
			Usage: map[string]any{
				"input_tokens":  12,
				"output_tokens": 34,
			},
		})

		v, ok := c.Get("onr.usage_input_tokens")
		if !ok || v != 12 {
			t.Fatalf("expected input_tokens=12, got ok=%v value=%v", ok, v)
		}
		v, ok = c.Get("onr.usage_output_tokens")
		if !ok || v != 34 {
			t.Fatalf("expected output_tokens=34, got ok=%v value=%v", ok, v)
		}
	})
}

func TestSetProxyResultContext_StreamPerfFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	setProxyResultContext(c, &proxy.Result{
		Status:    http.StatusOK,
		TTFTMs:    321,
		TPS:       12.5,
		LatencyMs: 1000,
	})

	if v, ok := c.Get("onr.ttft_ms"); !ok || v != int64(321) {
		t.Fatalf("expected ttft_ms=321, got ok=%v value=%v", ok, v)
	}
	if v, ok := c.Get("onr.tps"); !ok || v != 12.5 {
		t.Fatalf("expected tps=12.5, got ok=%v value=%v", ok, v)
	}
}
