package onrserver

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerWithColor_LogsAppNameFromHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	l := log.New(&out, "", 0)
	requestIDHeaderKey := "X-Onr-Request-Id"

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(requestIDHeaderKey, "rid-1")
		c.Next()
	})
	r.Use(requestLoggerWithColor(l, false, requestIDHeaderKey, false, ""))
	r.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("appname", "demo-client")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d", http.StatusOK, w.Code)
	}
	logLine := out.String()
	if !strings.Contains(logLine, "appname=demo-client") {
		t.Fatalf("expected appname in log, got=%q", logLine)
	}
}

func TestRequestLoggerWithColor_InfersAppNameFromUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	l := log.New(&out, "", 0)
	requestIDHeaderKey := "X-Onr-Request-Id"

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(requestIDHeaderKey, "rid-1")
		c.Next()
	})
	r.Use(requestLoggerWithColor(l, false, requestIDHeaderKey, true, ""))
	r.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("User-Agent", "claude-code/1.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logLine := out.String()
	if !strings.Contains(logLine, "appname=claude-code") {
		t.Fatalf("expected inferred appname in log, got=%q", logLine)
	}
}

func TestRequestLoggerWithColor_AppNameHeaderOverridesUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	l := log.New(&out, "", 0)
	requestIDHeaderKey := "X-Onr-Request-Id"

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(requestIDHeaderKey, "rid-1")
		c.Next()
	})
	r.Use(requestLoggerWithColor(l, false, requestIDHeaderKey, true, ""))
	r.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("appname", "manual-client")
	req.Header.Set("User-Agent", "claude-code/1.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logLine := out.String()
	if !strings.Contains(logLine, "appname=manual-client") {
		t.Fatalf("expected header appname in log, got=%q", logLine)
	}
	if strings.Contains(logLine, "appname=claude-code") {
		t.Fatalf("expected user-agent inference skipped when header exists, got=%q", logLine)
	}
}

func TestRequestLoggerWithColor_UsesUnknownWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	l := log.New(&out, "", 0)
	requestIDHeaderKey := "X-Onr-Request-Id"

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(requestIDHeaderKey, "rid-1")
		c.Next()
	})
	r.Use(requestLoggerWithColor(l, false, requestIDHeaderKey, true, "unknown-client"))
	r.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logLine := out.String()
	if !strings.Contains(logLine, "appname=unknown-client") {
		t.Fatalf("expected unknown fallback appname in log, got=%q", logLine)
	}
}

func TestRequestLoggerWithColor_LogsTTFTAndTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	l := log.New(&out, "", 0)
	requestIDHeaderKey := "X-Onr-Request-Id"

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(requestIDHeaderKey, "rid-1")
		c.Set("onr.ttft_ms", int64(123))
		c.Set("onr.tps", 45.67)
		c.Next()
	})
	r.Use(requestLoggerWithColor(l, false, requestIDHeaderKey, false, ""))
	r.GET("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logLine := out.String()
	if !strings.Contains(logLine, "ttft_ms=123") {
		t.Fatalf("expected ttft_ms in log, got=%q", logLine)
	}
	if !strings.Contains(logLine, "tps=45.67") {
		t.Fatalf("expected tps in log, got=%q", logLine)
	}
}
