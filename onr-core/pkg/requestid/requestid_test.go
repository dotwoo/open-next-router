package requestid

import (
	"regexp"
	"testing"
)

func TestGenFormat(t *testing.T) {
	id := Gen()
	if len(id) != 28 {
		t.Fatalf("unexpected len=%d id=%q", len(id), id)
	}
	if ok, _ := regexp.MatchString(`^[0-9]{28}$`, id); !ok {
		t.Fatalf("unexpected id format: %q", id)
	}
}

func TestHelpers(t *testing.T) {
	if got := randomDigits(0); got != "" {
		t.Fatalf("randomDigits(0)=%q", got)
	}
	s := randomDigits(12)
	if len(s) != 12 {
		t.Fatalf("randomDigits len=%d", len(s))
	}
	if ok, _ := regexp.MatchString(`^[0-9]{12}$`, s); !ok {
		t.Fatalf("randomDigits content=%q", s)
	}
	if got := cryptoRandIntn(0); got != 0 {
		t.Fatalf("cryptoRandIntn(0)=%d", got)
	}
	if got := timeString(); len(got) != 20 {
		t.Fatalf("timeString len=%d value=%q", len(got), got)
	}
}
