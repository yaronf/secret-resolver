# secret-resolver

POC: resolve Mamori URIs via a thin host that loads **out-of-process provider plugins** over a dedicated RPC fd — no cloud/provider SDKs in the resolver module.

**Platform:** Unix only for this POC (Linux, macOS, …) — `socketpair` + `ExtraFiles`. Windows alternatives (loopback + env token, or UDS / named pipe) are sketched in [docs/design-note.md](docs/design-note.md#transport), not implemented.

See [docs/plan.md](docs/plan.md) for goals and [docs/design-note.md](docs/design-note.md) for the maintainer-facing writeup.

## Quick demo (sqlite)

**You do not need `add-provider` for this.** Sqlite (and aws) are already in [`providers.manifest.json`](providers.manifest.json); committed generated mains are under `cmd/mamori-provider-sqlite/`.

Needs Mamori’s `Providers()` export (not in [upstream](https://github.com/xavidop/mamori) yet). Provider `go.mod` files `replace` that module to a sibling checkout named `mamori.git`:

```bash
git clone -b export-providers https://github.com/yaronf/mamori.git ../mamori.git
```

```bash
# optional: regenerate mains from the manifest
go generate .

mkdir -p bin
go build -o bin/secret-resolver ./cmd/secret-resolver

(cd cmd/mamori-provider-sqlite && go mod tidy && go build -o ../../bin/mamori-provider-sqlite .)

(cd examples/seed-sqlite && go mod tidy && go run . -db /tmp/mamori-demo.db)

./bin/secret-resolver \
  -provider ./bin/mamori-provider-sqlite \
  -env SQLITE_PATH=/tmp/mamori-demo.db \
  'sqlite://config/greeting'
```

Library usage: `secretresolver.New(secretresolver.WithProviders(...))`. If this lands in Mamori, provider lists belong in **Mamori’s YAML**, not a parallel schema here.

Root `go.mod` stays free of Mamori and provider SDKs.

## Adding another provider

Use **`add-provider` only when wrapping a package that is not in the manifest yet** (e.g. vault). It appends `name` + `import` and runs `go generate`.

```bash
./scripts/add-provider github.com/xavidop/mamori/providers/vault
(cd cmd/mamori-provider-vault && go mod tidy && go build)
```

For packages already listed, just `go generate .` (or use the committed `main.go`). The generated main blank-imports the package and calls `serve.ServeRegistered()` — schemes come from `init` / `Register`. That needs Mamori’s `Providers()` export (~10-line fix).

Fake provider is hand-written (not a Mamori package).

## Layout

| Path | Role |
| --- | --- |
| `.` (this module) | `Resolver`, RPC client — minimal deps |
| `serve/` | Provider-side `ServeRegistered` shim (depends on Mamori) |
| `cmd/secret-resolver` | CLI over the API |
| `providers.manifest.json` | which provider packages to wrap |
| `scripts/add-provider` | append a **new** package to the manifest + generate |
| `go generate .` | regenerate mains from the current manifest |
| `cmd/mamori-provider-sqlite` | Generated wrapper (already in manifest) |
| `cmd/mamori-provider-aws` | Generated wrapper (already in manifest) |
| `cmd/mamori-provider-fake` | Hand-written in-memory provider for tests |
