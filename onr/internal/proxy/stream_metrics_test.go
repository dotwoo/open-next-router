package proxy

import (
	"testing"
	"time"
)

func TestStreamPerfMetrics(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	first := start.Add(500 * time.Millisecond)

	ttft, tps := streamPerfMetrics(start, first, map[string]any{
		"output_tokens": 30,
	})
	if ttft != 500 {
		t.Fatalf("expected ttft=500, got=%d", ttft)
	}
	if tps <= 0 {
		t.Fatalf("expected tps>0, got=%f", tps)
	}
}

func TestStreamPerfMetrics_MissingOutputTokens(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	first := start.Add(300 * time.Millisecond)

	ttft, tps := streamPerfMetrics(start, first, map[string]any{})
	if ttft != 300 {
		t.Fatalf("expected ttft=300, got=%d", ttft)
	}
	if tps != 0 {
		t.Fatalf("expected tps=0, got=%f", tps)
	}
}
