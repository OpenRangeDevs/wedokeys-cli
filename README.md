# wedokeys-cli (`wdk`)

The command-line client for [WeDoKeys](https://app.wedokeys.com) — fetch your project's
secrets at runtime instead of copying them into `.env` files. `wdk` authenticates with a
service-account token, resolves the aliases listed in your project's `wdk.yml`, and injects
them as environment variables for local dev, scripts, and Kamal deploys.

`wdk` is a single self-contained Go binary — no Ruby runtime required. The original Ruby
implementation is kept as a reference in [`ruby-legacy/`](ruby-legacy/) and will be retired once the
Go CLI is proven.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/OpenRangeDevs/wedokeys-cli/main/install.sh | sh
```

This installs `wdk` (and the `kamal-secrets-wedokeys` adapter) into `~/.local/bin` — override with
`WDK_INSTALL_DIR` — and sets up **tab completion** for your shell where it can do so safely
(oh-my-zsh, bash, fish; it never edits rc files — otherwise it prints the one-liner to run).
macOS and Linux, amd64 and arm64. Or grab a binary from the
[releases page](https://github.com/OpenRangeDevs/wedokeys-cli/releases), or `go install
github.com/OpenRangeDevs/wedokeys-cli/cmd/wdk@latest` (then see `wdk completion --help`).

## Usage

Two guided, one-time steps — no file editing:

```sh
wdk login          # prompts for the server (defaults to production) + token
wdk init           # in your project: pick the project + secrets → writes wdk.yml
```

Then run things with secrets injected:

```sh
WDK_ENV=production wdk env exec -- bin/rails server    # run with secrets injected
wdk env export --env production                        # print export statements (direnv, scripts)
wdk subshell -e production                             # interactive shell with secrets loaded
```

Both setup commands take flags for automation:

```sh
wdk login --api-url http://localhost:3000 --token wdk_sat_...
wdk init --project my-app --secret STRIPE_KEY --secret POSTGRES_PASSWORD   # or --all
```

Every command has `--help`. `wdk init` lists the projects/aliases your token can see, so you never
copy slugs or alias names by hand.

The token is stored in `~/.wedokeys/config.yml` (mode 0600). The environment comes from `--env`,
`WDK_ENV`, or `KAMAL_DESTINATION`. If any alias fails to resolve the command exits non-zero and
nothing runs with partial secrets; pass `--allow-missing` to override. For Kamal, the
`kamal-secrets-wedokeys` adapter wraps `wdk kamal-fetch` — see
[`ruby-legacy/README.md`](ruby-legacy/README.md) for the `.kamal/secrets` snippet.

## Documentation

- [`docs/API.md`](docs/API.md) — the `/api/v1/resolve` request/response contract and error codes.
- [`docs/LOCAL_TESTING.md`](docs/LOCAL_TESTING.md) — running the whole loop against a local server.

## Development

```sh
go build ./cmd/wdk        # build the binary
go test ./...             # unit tests
gofmt -l . && go vet ./...
```

### Parity with the Ruby reference

While both implementations are maintained, any change to CLI behavior must land in **both** the Go
code and [`ruby-legacy/`](ruby-legacy/). The parity harness runs the Go binary and the Ruby CLI
against the same stub server and asserts identical stdout/stderr/exit code:

```sh
( cd ruby-legacy && bundle install )   # once
go test -tags parity ./test/parity/
```

## Related

The WeDoKeys application and API (the server this CLI talks to) is maintained separately.
This CLI is published here, in the open, so its handling of your tokens and secrets is fully
inspectable.
