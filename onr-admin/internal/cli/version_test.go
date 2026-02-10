package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	sharedver "github.com/r9s-ai/open-next-router/internal/version"
)

func TestVersionCmdOutput(t *testing.T) {
	t.Parallel()

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version cmd: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace(fmt.Sprint(sharedver.Get()))
	if got != want {
		t.Fatalf("version output=%q want=%q", got, want)
	}
}

func TestRootCmdHasVersionSubcommand(t *testing.T) {
	t.Parallel()

	root := newRootCmd()
	_, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("find version subcommand: %v", err)
	}
}
