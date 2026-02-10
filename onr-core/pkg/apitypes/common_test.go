package apitypes

import (
	"testing"
)

func TestParseJSONObject(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		obj, err := ParseJSONObject([]byte(`{"a":1,"b":"x"}`), "payload")
		if err != nil {
			t.Fatalf("ParseJSONObject err=%v", err)
		}
		if got := obj["b"]; got != "x" {
			t.Fatalf("unexpected field b: %v", got)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		if _, err := ParseJSONObject([]byte(`{`), "payload"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("not object", func(t *testing.T) {
		if _, err := ParseJSONObject([]byte(`[1,2,3]`), "payload"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestJSONObjectMarshal(t *testing.T) {
	b, err := JSONObject{
		"x": "y",
		"n": 1,
	}.Marshal()
	if err != nil {
		t.Fatalf("Marshal err=%v", err)
	}
	if len(b) == 0 {
		t.Fatalf("marshal returned empty bytes")
	}
}
