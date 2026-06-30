# Testing WeDoKeys locally (web app + wdk CLI, end to end)

This walkthrough stands up the Rails app locally, creates demo data, and
drives the wdk CLI from a fake consumer project — proving the full loop:
admin creates aliases and a service account in the UI → a developer's
project resolves secrets at runtime.

Time: ~10 minutes. Nothing here touches your real `~/.wedokeys`.

## 1. App setup (one-time)

Follow [SETUP.md](SETUP.md) for prerequisites (Ruby 3.3.6, PostgreSQL). Then:

```sh
cd /path/to/wedokeys
bundle install
bin/rails db:prepare
```

## 2. Start the server

```sh
bin/dev
```

Leave this running (Rails on http://localhost:3000 + Tailwind watcher).

## 3. Create demo data

In a second terminal, from the repo root:

```sh
bin/rails wdk:demo:setup
```

This prints everything the rest of this guide uses:
- Web UI login: `demo-owner@wdk.test` / `demo-password-123`
- Two project slugs: `demo-store` and `demo-analytics`
- Two static tokens: one **project-scoped** to `demo-store`, one **account-level**
- A ready-made `wdk.yml`

Rerunnable: it tears down and recreates the "WDK Demo" account each time
(`bin/rails wdk:demo:teardown` removes it entirely).

## 4. Install the CLI

The `wdk` CLI is a self-contained Go binary
([OpenRangeDevs/wedokeys-cli](https://github.com/OpenRangeDevs/wedokeys-cli)):

```sh
curl -fsSL https://raw.githubusercontent.com/OpenRangeDevs/wedokeys-cli/main/install.sh | sh
wdk version   # sanity check
```

## 5. Log in (pointed at localhost, in an isolated HOME)

The CLI reads `$HOME/.wedokeys/config.yml`. Use a throwaway HOME so the demo
can't touch a real config; `--api-url` points it at the local server (no file
editing):

```sh
export DEMO_HOME=$(mktemp -d)
HOME="$DEMO_HOME" wdk login --api-url http://localhost:3000 --token <PROJECT-SCOPED TOKEN FROM STEP 3>
```

Expected: `Logged in. Token saved to ~/.wedokeys/config.yml`. (Interactively,
plain `wdk login` prompts for the server and token.)

> Prefix every `wdk` command below with `HOME="$DEMO_HOME"`, or
> `export HOME="$DEMO_HOME"` in a dedicated terminal for the demo.

## 6. Configure the consumer project

Anywhere **outside** the wedokeys repo, let `wdk init` write `wdk.yml` for you —
it auto-selects the project the token is scoped to and lists its aliases:

```sh
mkdir -p ~/Code/wdk-demo-app && cd ~/Code/wdk-demo-app
HOME="$DEMO_HOME" wdk init --all     # → project: demo-store, secrets: [STRIPE_KEY]
# or interactively: HOME="$DEMO_HOME" wdk init
```

## 7. The demo

All commands run from `~/Code/wdk-demo-app` with `HOME="$DEMO_HOME"`.

**Happy path — secrets injected into a process:**

```sh
WDK_ENV=development wdk env exec -- printenv STRIPE_KEY
# => sk_demo_stripe_12345        (exit 0)

wdk env export --env development
# => export STRIPE_KEY=sk_demo_stripe_12345
```

**Fail-hard on an unresolvable alias** (`INACTIVE_KEY` exists but is
deactivated). Add it to `wdk.yml`'s secrets list, then:

```sh
WDK_ENV=development wdk env exec -- printenv STRIPE_KEY; echo "exit=$?"
# stderr => INACTIVE_KEY: Reference is not active (inactive_reference)
# stderr => Error: 1 of 2 secrets could not be resolved. ...
# => exit=1   (the command did NOT run)

WDK_ENV=development wdk env exec --allow-missing -- printenv STRIPE_KEY
# => sk_demo_stripe_12345        (proceeds with what resolved)
```

**Wrong environment** — replace `INACTIVE_KEY` with `STAGING_ONLY` in
`wdk.yml`:

```sh
WDK_ENV=development wdk env exec -- true; echo "exit=$?"
# stderr => STAGING_ONLY: Reference environment does not match request (environment_mismatch)
# => exit=1
```

**Project scoping** — the token is scoped to `demo-store`. Point `wdk.yml`
at the other project (`project: demo-analytics`, secrets: `ANALYTICS_KEY`):

```sh
WDK_ENV=development wdk env exec -- true; echo "exit=$?"
# stderr => ANALYTICS_KEY: Service account is not scoped to this project (project_scope_denied)
# => exit=1
```

Log in with the **account-level** token from step 3 and rerun — it resolves:

```sh
HOME="$DEMO_HOME" wdk login --token <ACCOUNT-LEVEL TOKEN>
WDK_ENV=development wdk env exec -- printenv ANALYTICS_KEY
# => an_demo_key_12345           (exit 0)
```

**Kamal adapter:**

```sh
wdk kamal-fetch --from demo-store/development STRIPE_KEY
# => STRIPE_KEY=sk_demo_stripe_12345
```

## 8. Verify the audit trail

Every denial above (except unknown names) wrote a `SecretAccessEvent`:

```sh
# from the wedokeys repo
bin/rails runner 'SecretAccessEvent.order(:id).last(10).each { |e| puts [e.result, e.reason_code, e.secret_reference&.alias_name].compact.join("  ") }'
```

Expected to include `success` rows plus `denied inactive_reference INACTIVE_KEY`,
`denied environment_mismatch STAGING_ONLY`, `denied project_scope_denied ANALYTICS_KEY`.

## 9. UI walkthrough (http://localhost:3000)

Log in as `demo-owner@wdk.test` / `demo-password-123` and check:

- **Dashboard** → expand a project: alias rows show env badges
  (production red / staging yellow / development green); "Remove" asks for
  confirmation before deleting.
- **Add Project**: typing a Name live-fills the Slug; editing the slug by
  hand stops the auto-fill. **Edit** an existing project: the slug is
  read-only, and renaming the project does not change it.
- **wdk (header) → service account → Delete / Revoke**: a confirmation
  dialog appears (Turbo confirm).
- **Issue token** on a service account: the token banner's 📋 Copy button
  copies to the clipboard.

## 10. Cleanup

```sh
bin/rails wdk:demo:teardown
rm -rf ~/Code/wdk-demo-app "$DEMO_HOME"
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `Authentication error` on every call | Token pasted with whitespace, or demo data recreated (tokens rotate on each `wdk:demo:setup`) — re-run `wdk login` with the freshly printed token |
| `API error: environment does not match service account environment` | Demo tokens are `development`-only; use `WDK_ENV=development` |
| `API error: Project not found` | `project:` in wdk.yml must be a printed slug (`demo-store` / `demo-analytics`) |
| CLI hits `app.wedokeys.com` | `$HOME` is not the demo HOME, or `api_url` missing from `$DEMO_HOME/.wedokeys/config.yml` |
| `No wdk.yml found` | Run `wdk init` (or `wdk env …`) from the consumer project directory (e.g. `~/Code/wdk-demo-app`), not the wedokeys repo |

CLI usage reference: [OpenRangeDevs/wedokeys-cli](https://github.com/OpenRangeDevs/wedokeys-cli).
API contract: [API.md](API.md).
