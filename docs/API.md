# WeDoKeys API Contract

Programmatic access for service accounts. All endpoints are JSON over HTTPS and
authenticate with a bearer token (`Authorization: Bearer <token>`).

## POST /api/v1/resolve

Resolves secret values at runtime. Two mutually exclusive request modes —
sending both in one request is rejected with `mixed_modes`.

### Aliases mode (used by the wdk CLI)

```json
{
  "aliases": ["STRIPE_KEY", "POSTGRES_PASSWORD"],
  "project": "my-app",
  "environment": "production"
}
```

- `project` is the project **slug** (shown on the project form; the value in `wdk.yml`).
- Resolved values are keyed by alias name.

### References mode

```json
{
  "references": ["wdk://ref/sr_abc123", "sr_def456"],
  "environment": "production"
}
```

- Accepts canonical `wdk://ref/<key>` URIs or bare `sr_` reference keys.
- Resolved values are keyed by canonical URI.

### Response (200)

Resolution is **per-item**: a request can partially succeed. Items that could
not be resolved appear in `errors`, never in `resolved`.

```json
{
  "resolved": { "STRIPE_KEY": "sk_live_..." },
  "errors": [
    { "reference": "OLD_KEY", "code": "inactive_reference", "message": "Reference is not active" }
  ],
  "ttl_seconds": 300,
  "request_id": "req_..."
}
```

### Top-level error codes (HTTP 400)

| Code | Meaning |
|------|---------|
| `missing_environment` | `environment` param absent |
| `invalid_environment` | not one of `development`, `staging`, `production` |
| `environment_mismatch` | request environment differs from the service account's environment |
| `mixed_modes` | both `references` and `aliases` were sent |
| `missing_references` | references mode with no references |
| `missing_aliases` | aliases mode with an absent or empty `aliases` array |
| `missing_project` | aliases mode without a `project` slug |
| `project_not_found` | no project with that slug in the token's account |

### Per-item denial codes (in `errors`, HTTP 200)

Checked in this order; every denial except `not_found` writes a denied
`SecretAccessEvent` audit record with the matching `reason_code`.

| Code | Meaning |
|------|---------|
| `not_found` | no reference/alias with that identifier — **or** a reference that exists in another account (these are deliberately indistinguishable; see below) |
| `project_scope_denied` | the service account is scoped to a specific project and this reference belongs to another (checked before active/environment so scoped accounts cannot probe other projects' references) |
| `inactive_reference` | reference is deactivated |
| `environment_mismatch` | reference exists but for a different environment |
| `scope_not_allowed` | the token's scopes exclude this reference key |

A reference key belonging to a **different account** returns `not_found` — identical to a key that
doesn't exist — so the endpoint can't be used to confirm cross-account key existence. The attempt is
still recorded internally with the audit reason code `account_boundary` (visible in Access Logs),
which is never returned to the client.

### Authentication failures

`401` with no resolution. The wdk CLI maps 401 to "run `wdk login`"; any 400 is
reported as an API error. (`wdk login` verifies tokens by sending an
intentionally invalid resolve request and treating any non-401 as success —
the endpoint must keep returning 400, not 401, for an authenticated request
with bad params.)

## GET /api/v1/projects

Lists the projects the token can access (used by `wdk init`). An account-level
token sees every project in its account; a project-scoped token sees only its
own project. Same bearer auth as `/resolve`; `401` when the token is
missing/invalid.

### Response (200)

```json
{
  "projects": [ { "slug": "my-app", "name": "My App" } ],
  "request_id": "req_..."
}
```

## GET /api/v1/projects/:slug/aliases

Lists the **active** aliases of a project in the **token's environment** — i.e.
exactly the aliases that would resolve for this token (used by `wdk init`).
Aliases without a name, inactive ones, and other environments are omitted.

### Response (200)

```json
{
  "aliases": [ { "name": "STRIPE_KEY", "environment": "development" } ],
  "request_id": "req_..."
}
```

### Errors (HTTP 400)

| Code | Meaning |
|------|---------|
| `project_scope_denied` | a project-scoped token requested a different (or unknown) project — checked **before** not-found so it can't probe whether other projects exist |
| `project_not_found` | an account-level token requested a slug that doesn't exist |

## POST /api/v1/token/exchange

Exchanges a static token (`wdk_sat_…`) for a short-lived ephemeral token
(`wdk_rt_…`) with `grant_type: "urn:wedokeys:grant:static_token"`. See
`Api::V1::TokensController`.
