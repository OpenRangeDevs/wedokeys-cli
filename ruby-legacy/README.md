# wdk — the WeDoKeys CLI

Fetch your project's secrets at runtime instead of copying them into `.env`
files. `wdk` authenticates with a service-account token, resolves the aliases
listed in your project's `wdk.yml`, and injects them as environment variables.

## Install

```sh
# Homebrew (recommended)
brew install OpenRangeDevs/tap/wdk

# or RubyGems (Ruby 3.1+)
gem install wdk
```

<details>
<summary>Build from source</summary>

```sh
cd cli && gem build wdk.gemspec && gem install wdk-*.gem
```
</details>

## 1. Get a token

An owner/admin creates these in the WeDoKeys web UI:

1. **Project slug** — shown on the project's form; this exact value goes in `wdk.yml`.
2. **Aliases** — Dashboard → project → *wdk Aliases* → *Add Alias* (e.g. `STRIPE_KEY` pointing at a secret field, per environment).
3. **Service account + token** — header → *wdk* → *New service account* (pick the environment, and optionally restrict it to one project), then issue a token. The token is shown **once**.

## 2. Log in

```sh
wdk login            # paste the token when prompted
wdk login --token wdk_sat_...
```

The token is stored in `~/.wedokeys/config.yml` (mode 0600). To target a
non-default server (e.g. local development), add `api_url` to that file:

```yaml
token: wdk_sat_...
api_url: http://localhost:3000
```

## 3. Configure your project

Create `wdk.yml` in the project root (it is found upward from the current
directory, like `.git`):

```yaml
project: my-app          # the project slug from the web UI
secrets:
  - STRIPE_KEY
  - POSTGRES_PASSWORD
```

The environment comes from `--env`, `WDK_ENV`, or `KAMAL_DESTINATION`
(checked in that order) and must match the service account's environment.

## 4. Run things

```sh
WDK_ENV=production wdk env exec -- bin/rails server   # run with secrets injected
wdk env export --env production                       # print export statements (direnv, scripts)
wdk subshell -e production                            # interactive shell with secrets loaded
```

If **any** requested alias fails to resolve, the command prints each failure
and exits 1 — nothing runs with partial secrets. Pass `--allow-missing` to
proceed anyway.

## Kamal adapter

`bin/kamal-secrets-wedokeys` wraps `wdk kamal-fetch`. In `.kamal/secrets`:

```sh
SECRETS=$(kamal secrets fetch --adapter wedokeys --from my-app/production STRIPE_KEY POSTGRES_PASSWORD)
STRIPE_KEY=$(kamal secrets extract STRIPE_KEY $SECRETS)
```

`--from` takes `project/environment`; without it, `wdk.yml` +
`KAMAL_DESTINATION` are used. The fetch is all-or-nothing: a missing alias
fails the deploy at fetch time.

## Troubleshooting

| Message | Cause | Fix |
|---------|-------|-----|
| `No token found. Run \`wdk login\` first.` | no `~/.wedokeys/config.yml` | `wdk login` |
| `Invalid token — authentication failed.` | token revoked/expired/mistyped | issue a new token in the web UI, `wdk login` |
| `Authentication failed. Run \`wdk login\` to refresh your token.` | token became invalid mid-use | same as above |
| `No wdk.yml found.` | running outside a configured project | create `wdk.yml` with `project:` and `secrets:` |
| `Environment not set.` | no `--env` / `WDK_ENV` / `KAMAL_DESTINATION` | set one of them |
| `API error: environment does not match service account environment` | token's service account is for a different environment | use the matching token, or fix the env setting |
| `API error: Project not found` | `project:` in `wdk.yml` doesn't match any slug in your account | copy the slug exactly from the project form |
| `<ALIAS>: Alias not found (not_found)` | alias doesn't exist for that project+environment | add it in the web UI, or fix the name in `wdk.yml` |
| `<ALIAS>: Reference is not active (inactive_reference)` | an admin deactivated the alias | re-activate it or remove it from `wdk.yml` |
| `<ALIAS>: Reference environment does not match request (environment_mismatch)` | alias exists for another environment | create the alias for this environment |
| `<ALIAS>: Service account is not scoped to this project (project_scope_denied)` | the token's service account is restricted to a different project | use a token for this project, or an account-level service account |
| `<ALIAS>: Token scope does not allow this reference (scope_not_allowed)` | token has restricted scopes | issue a token with the needed scope |
| `N of M secrets could not be resolved.` | one or more aliases failed (see lines above it) | fix them, or `--allow-missing` to proceed without |

The full API contract (request/response shapes, every error code) is in
[documentation/API.md](../documentation/API.md). To test the whole loop
against a local server with demo data, follow
[documentation/LOCAL_TESTING.md](../documentation/LOCAL_TESTING.md).
