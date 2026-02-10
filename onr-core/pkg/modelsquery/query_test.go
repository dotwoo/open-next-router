package modelsquery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestQuery_CustomModelsConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
	}))
	defer srv.Close()

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			BaseURLExpr: `"` + srv.URL + `"`,
		},
		Headers: dslconfig.ProviderHeaders{
			Defaults: dslconfig.PhaseHeaders{
				Auth: []dslconfig.HeaderOp{
					{
						Op:        "header_set",
						NameExpr:  `"Authorization"`,
						ValueExpr: `concat("Bearer ", $channel.key)`,
					},
				},
			},
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	result, err := Query(nil, Params{
		Provider: "openai",
		File:     pf,
		Meta: dslmeta.Meta{
			API: "chat.completions",
		},
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.IDs) != 2 || result.IDs[0] != "gpt-4o-mini" || result.IDs[1] != "gpt-4.1" {
		t.Fatalf("ids=%v", result.IDs)
	}
}
