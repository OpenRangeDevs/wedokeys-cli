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

## 4. Set up the CLI (no install needed)

Resolve the real ruby binary first — version-manager shims (asdf, rbenv)
silently fail with exit 126 once `$HOME` is overridden in step 5:

```sh
export WDK_REPO=/path/to/wedokeys
export WDK_RUBY=$(ruby -e 'puts RbConfig.ruby')
wdk() { "$WDK_RUBY" -I "$WDK_REPO/cli/lib" "$WDK_REPO/cli/bin/wdk" "$@"; }
wdk help   # sanity check — prints the command list
```

(Alternative: `cd cli && gem build wdk.gemspec && gem install wdk-*.gem`.)

## 5. Point the CLI at localhost, in an isolated HOME

The CLI reads `$HOME/.wedokeys/config.yml`. Use a throwaway HOME so the demo
can't touch a real config:

```sh
export DEMO_HOME=$(mktemp -d)
mkdir -p "$DEMO_HOME/.wedokeys"
cat > "$DEMO_HOME/.wedokeys/config.yml" <<EOF
api_url: http://localhost:3000
EOF
HOME="$DEMO_HOME" wdk login --token <PROJECT-SCOPED TOKEN FROM STEP 3>
```

Expected: `Logged in. Token saved to ~/.wedokeys/config.yml` — and the file
still contains the `api_url` line (login merges, it does not overwrite).

> Prefix every `wdk` command below with `HOME="$DEMO_HOME"`, or
> `export HOME="$DEMO_HOME"` in a dedicated terminal for the demo.

## 6. Create the fake consumer project

Anywhere **outside** the wedokeys repo:

```sh
mkdir -p ~/Code/wdk-demo-app && cd ~/Code/wdk-demo-app
cat > wdk.yml <<EOF
project: demo-store
secrets:
  - STRIPE_KEY
EOF
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
cd $WDK_REPO
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
| `wdk` exits 126 with no output | You're calling a ruby version-manager shim with `$HOME` overridden — use the `WDK_RUBY` function from step 4 |
| `No wdk.yml found` | Run from `~/Code/wdk-demo-app` |

CLI usage reference: [cli/README.md](../cli/README.md). API contract: [API.md](API.md).
