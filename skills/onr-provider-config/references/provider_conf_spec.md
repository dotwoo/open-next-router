# Provider Config Spec

Use this JSON schema guide with `scripts/render_provider_conf.py`.

## Required fields

- `provider`: lowercase provider name, used as `<provider>.conf`
- `base_url`: upstream base URL
- `routes`: list of match rules (or use `preset` to auto-fill)

## Optional top-level fields

- `preset`: currently supports `openai-compatible`
- `auth`: defaults auth block
- `request`: defaults request block
- `response`: defaults response block
- `metrics`: defaults metrics block
- `error_map`: defaults error block (`openai` etc.)
- `balance`: defaults balance block
- `models`: defaults models block

Balance object note:
- use `balance_expr` to render `balance_expr = <expr>;`

## Route object

Each route in `routes` supports:

- `api`: DSL api name (for example `chat.completions`)
- `stream`: optional boolean
- `path` or `path_expr`: upstream path configuration
- `set_query`: object of query key -> value
- `del_query`: list of query keys
- `auth`, `request`, `response`, `metrics`, `error_map`: match-level overrides
- `extra_directives`: raw DSL statements (strings)

## Expression encoding

For most value fields, use one of:

- JSON string/number/bool: rendered as DSL literal
- `{"expr": "..."}`: rendered as raw DSL expression

Example:

- `"foo"` -> `"foo"`
- `true` -> `true`
- `{"expr":"$request.model_mapped"}` -> `$request.model_mapped`

## Example 1: OpenAI-compatible quick scaffold

```json
{
  "provider": "acme-openai",
  "base_url": "https://api.acme.ai",
  "preset": "openai-compatible"
}
```

## Example 2: Custom header auth + explicit routes

```json
{
  "provider": "my-anthropic-proxy",
  "base_url": "https://api.anthropic.com",
  "auth": {
    "mode": "header",
    "header": "x-api-key"
  },
  "response": {
    "mode": "passthrough"
  },
  "metrics": {
    "usage_extract": "anthropic",
    "finish_reason_extract": "anthropic"
  },
  "models": {
    "mode": "custom",
    "path": "/v1/models",
    "id_path": [
      "$.data[*].id"
    ]
  },
  "routes": [
    {
      "api": "claude.messages",
      "path": "/v1/messages",
      "request": {
        "json_del": [
          "$.stream_options"
        ]
      }
    }
  ]
}
```

## Validation after generation

Run from repository root:

```bash
go test ./onr-core/pkg/dslconfig -run TestValidateProvidersDir_ConfigProviders
```
