---
name: onr-provider-config
description: Generate and update ONR provider DSL configs in config/providers/*.conf from user requirements. Use when Codex is asked to add a new provider, scaffold provider configuration files, convert API/auth/base_url requirements into explicit DSL directives, or validate provider config generation.
---

# ONR Provider Config

## Overview

Create provider configuration files for ONR under `config/providers/` using explicit DSL directives. Generate from structured user input, keep filename/provider-name consistency, and validate that the providers directory remains parseable.

## Workflow

1. Collect minimum required inputs before writing files.
- `provider` (lowercase, hyphen-safe)
- `base_url`
- auth style (`bearer`, header key, or OAuth)
- supported APIs and upstream paths
- optional blocks (`metrics`, `models`, `balance`, `response`, `error`)

2. Build a JSON spec and render with script.
- Create a temporary JSON spec that matches `references/provider_conf_spec.md`.
- Run:
```bash
python3 skills/onr-provider-config/scripts/render_provider_conf.py \
  --spec /tmp/provider-spec.json \
  --output-dir config/providers \
  --overwrite
```

3. Review and adjust generated DSL if needed.
- Confirm file path is `config/providers/<provider>.conf`.
- Confirm provider block name matches filename exactly.
- Confirm all compatibility behavior is explicit (`req_map`, `resp_map`, `sse_parse`) when needed.

4. Validate repository state.
- Run provider directory validation test:
```bash
go test ./onr-core/pkg/dslconfig -run TestValidateProvidersDir_ConfigProviders
```
- If changed directives are uncommon, also inspect `DSL_SYNTAX.md` and `config/providers/example-full.conf`.

5. Report concrete results.
- Mention created/updated file path.
- Summarize selected auth/match/routes semantics.
- Report validation command result.

## Guardrails

- Avoid implicit compatibility behavior in runtime code.
- Use only explicit DSL directives.
- Keep source/comments in English.
- Keep every directive terminated with `;`.
- Keep one provider per file.

## Resources

- Spec and field reference: `references/provider_conf_spec.md`
- Generator script: `scripts/render_provider_conf.py`
