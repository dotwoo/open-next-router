package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/keystore"
)

func TestResolveEncryptPlaintext_FromFlag(t *testing.T) {
	t.Parallel()

	got, err := resolveCryptoInput("  hello  ", bytes.NewBufferString("ignored"), true)
	if err != nil {
		t.Fatalf("resolveCryptoInput err=%v", err)
	}
	if got != "hello" {
		t.Fatalf("got=%q want=%q", got, "hello")
	}
}

func TestResolveEncryptPlaintext_FromTerminalLine(t *testing.T) {
	t.Parallel()

	got, err := resolveCryptoInput("", bytes.NewBufferString("secret\nsecond\n"), true)
	if err != nil {
		t.Fatalf("resolveCryptoInput err=%v", err)
	}
	if got != "secret" {
		t.Fatalf("got=%q want=%q", got, "secret")
	}
}

func TestResolveEncryptPlaintext_FromPipe(t *testing.T) {
	t.Parallel()

	got, err := resolveCryptoInput("", bytes.NewBufferString("secret\nsecond\n"), false)
	if err != nil {
		t.Fatalf("resolveCryptoInput err=%v", err)
	}
	if got != "secret\nsecond" {
		t.Fatalf("got=%q want=%q", got, "secret\nsecond")
	}
}

func TestCryptoDecryptCmd(t *testing.T) {
	t.Setenv("ONR_MASTER_KEY", "12345678901234567890123456789012")
	enc, err := keystore.Encrypt("hello")
	if err != nil {
		t.Fatalf("Encrypt err=%v", err)
	}

	cmd := newCryptoDecryptCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--text", enc})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute decrypt cmd: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "hello" {
		t.Fatalf("decrypt output=%q want=%q", got, "hello")
	}
}
