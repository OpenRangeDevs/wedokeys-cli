# wedokeys-cli (`wdk`)

The command-line client for [WeDoKeys](https://app.wedokeys.com) — fetch your project's
secrets at runtime instead of copying them into `.env` files. `wdk` authenticates with a
service-account token, resolves the aliases listed in your project's `wdk.yml`, and injects
them as environment variables for local dev, scripts, and Kamal deploys.

> **Status: Go port in progress.** This repository is the public home of the `wdk` CLI. The
> CLI is being ported from Ruby to a single self-contained Go binary (installable via a
> `curl … | sh` script — no Ruby runtime required). The original Ruby implementation is kept
> as a reference in [`ruby-legacy/`](ruby-legacy/) and will be retired once the Go CLI is
> proven. Install instructions land with the first release.

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
