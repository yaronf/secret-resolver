# mamori-resolver

POC: resolve Mamori URIs via a thin host that loads **out-of-process provider plugins** over stdio RPC — no cloud/provider SDKs in the resolver module.

See [docs/plan.md](docs/plan.md) for goals and [docs/design-note.md](docs/design-note.md) for the maintainer-facing writeup.

## Provider binaries

Add a Mamori provider package (updates [`providers.manifest.json`](providers.manifest.json) and runs `go generate`):

```bash
./scripts/add-provider github.com/xavidop/mamori/providers/vault
```

The manifest is just `name` + `import`. The generated main blank-imports that package and calls `serve.ServeRegistered()` — schemes come from whatever `init` registered. This needs Mamori’s `Providers()` export (this POC `replace`s a local mamori tree that has it).

Then:

```bash
cd cmd/mamori-provider-vault && go mod tidy && go build
```

Fake provider stays hand-written (not a Mamori package).

## Quick demo (sqlite)

```bash
# build
go build -o bin/mamori-resolver ./cmd/mamori-resolver
go build -o bin/mamori-provider-sqlite ./cmd/mamori-provider-sqlite

# seed a tiny DB
go run ./examples/seed-sqlite -db /tmp/mamori-demo.db

# resolve (API / flags — no separate config file format)
./bin/mamori-resolver \
  -provider ./bin/mamori-provider-sqlite \
  -env SQLITE_PATH=/tmp/mamori-demo.db \
  'sqlite://config/greeting'
```

Library usage is the same shape: `resolver.New(resolver.WithProviders(...))`. If this lands in Mamori, provider lists belong in **Mamori’s YAML**, not a parallel schema here.

`go.mod` at the repo root must stay free of `github.com/xavidop/mamori` and provider SDKs. Provider binaries live under `cmd/mamori-provider-*` / `serve/` and pull those deps themselves.

## Layout

| Path | Role |
| --- | --- |
| `.` (this module) | `Resolver`, RPC client — minimal deps |
| `serve/` | Provider-side `Serve(...)` shim (depends on Mamori) |
| `cmd/mamori-resolver` | CLI over the API |
| `scripts/add-provider` + `providers.manifest.json` | add providers → generate mains |
| `cmd/mamori-provider-sqlite` | Generated wrapper around Mamori sqlite |
| `cmd/mamori-provider-aws` | Generated wrapper (optional heavy-SDK demo) |
| `cmd/mamori-provider-fake` | Hand-written in-memory provider for tests |
