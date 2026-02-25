package onrserver

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestShouldTriggerProviderReload(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		if shouldTriggerProviderReload(fsnotify.Event{Name: "", Op: fsnotify.Write}) {
			t.Fatalf("expected false for empty event name")
		}
	})

	t.Run("unsupported op", func(t *testing.T) {
		if shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/a.conf", Op: 0}) {
			t.Fatalf("expected false for unsupported op")
		}
	})

	t.Run("dot file ignored", func(t *testing.T) {
		if shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/.a.conf", Op: fsnotify.Write}) {
			t.Fatalf("expected false for dotfile")
		}
	})

	t.Run("conf write", func(t *testing.T) {
		if !shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/a.conf", Op: fsnotify.Write}) {
			t.Fatalf("expected true for conf write")
		}
	})

	t.Run("remove", func(t *testing.T) {
		if !shouldTriggerProviderReload(fsnotify.Event{Name: "/tmp/a.conf", Op: fsnotify.Remove}) {
			t.Fatalf("expected true for remove")
		}
	})
}
