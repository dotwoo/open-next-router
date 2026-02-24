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
	r.Use(requestLoggerWithColor(l, false, requestIDHeaderKey))
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
