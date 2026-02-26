package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDumpSummary_Basic(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "a.log")
	content := `=== META ===
time=2026-02-09T12:29:48+08:00
request_id=2026020912294802709353375236
method=POST
path=/v1/chat/completions
client_ip=::1
headers:
  X-Onr-Provider: gemini
  Content-Type: application/json

=== ORIGIN REQUEST ===
{"model":"gemini-2.0-flash","stream":true,"messages":[{"role":"user","content":"count 1 to 3"}]}

=== PROXY RESPONSE ===
status=200

`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	sum, err := ParseDumpSummary(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.RequestID != "2026020912294802709353375236" {
		t.Fatalf("request_id mismatch: %q", sum.RequestID)
	}
	if sum.Method != "POST" {
		t.Fatalf("method mismatch: %q", sum.Method)
	}
	if sum.URLPath != "/v1/chat/completions" {
		t.Fatalf("path mismatch: %q", sum.URLPath)
	}
	if sum.Provider != "gemini" {
		t.Fatalf("provider mismatch: %q", sum.Provider)
	}
	if sum.Model != "gemini-2.0-flash" {
		t.Fatalf("model mismatch: %q", sum.Model)
	}
	if sum.Stream == nil || *sum.Stream != true {
		t.Fatalf("stream mismatch: %#v", sum.Stream)
	}
	if sum.ProxyStatus != 200 {
		t.Fatalf("status mismatch: %d", sum.ProxyStatus)
	}
	if sum.Time.IsZero() {
		t.Fatalf("time should be parsed")
	}
}

func TestListDumpSummaries_SortNewestFirst(t *testing.T) {
	tmp := t.TempDir()
	p1 := filepath.Join(tmp, "1.log")
	p2 := filepath.Join(tmp, "2.log")
	if err := os.WriteFile(p1, []byte("=== META ===\nrequest_id=1\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte("=== META ===\nrequest_id=2\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	old := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p1, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p2, newer, newer); err != nil {
		t.Fatal(err)
	}

	list, err := ListDumpSummaries(DumpListOptions{Dir: tmp, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	if filepath.Base(list[0].Path) != "2.log" {
		t.Fatalf("expected newest first, got %s", filepath.Base(list[0].Path))
	}
}

func TestFindDumpByRequestID_DirectFileHit(t *testing.T) {
	tmp := t.TempDir()
	rid := "rid-direct-1"
	p := filepath.Join(tmp, rid+".log")
	content := "=== META ===\nrequest_id=" + rid + "\n\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	sum, found, err := FindDumpByRequestID(DumpFindOptions{
		Dir:       tmp,
		RequestID: rid,
	})
	if err != nil {
		t.Fatalf("FindDumpByRequestID error: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if sum.Path != p {
		t.Fatalf("path mismatch: %q", sum.Path)
	}
	if sum.RequestID != rid {
		t.Fatalf("request_id mismatch: %q", sum.RequestID)
	}
}

func TestFindDumpByRequestID_FallbackMetaMatch(t *testing.T) {
	tmp := t.TempDir()
	rid := "rid-meta-1"
	p := filepath.Join(tmp, "custom-name.log")
	content := "=== META ===\nrequest_id=" + rid + "\n\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	sum, found, err := FindDumpByRequestID(DumpFindOptions{
		Dir:       tmp,
		RequestID: rid,
	})
	if err != nil {
		t.Fatalf("FindDumpByRequestID error: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if sum.Path != p {
		t.Fatalf("path mismatch: %q", sum.Path)
	}
	if sum.RequestID != rid {
		t.Fatalf("request_id mismatch: %q", sum.RequestID)
	}
}

func TestFindDumpByRequestID_NotFound(t *testing.T) {
	tmp := t.TempDir()
	_, found, err := FindDumpByRequestID(DumpFindOptions{
		Dir:       tmp,
		RequestID: "rid-not-found",
	})
	if err != nil {
		t.Fatalf("FindDumpByRequestID error: %v", err)
	}
	if found {
		t.Fatalf("expected found=false")
	}
}

func TestFindDumpByRequestID_ReturnsNewest(t *testing.T) {
	tmp := t.TempDir()
	rid := "rid-newest-1"
	oldPath := filepath.Join(tmp, "old.log")
	newPath := filepath.Join(tmp, "new.log")
	content := "=== META ===\nrequest_id=" + rid + "\n\n"
	if err := os.WriteFile(oldPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	sum, found, err := FindDumpByRequestID(DumpFindOptions{
		Dir:       tmp,
		RequestID: rid,
	})
	if err != nil {
		t.Fatalf("FindDumpByRequestID error: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if sum.Path != newPath {
		t.Fatalf("expected newest=%q got=%q", newPath, sum.Path)
	}
}
