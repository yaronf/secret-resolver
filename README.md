# mamori-resolver

POC: resolve Mamori URIs via a thin host that loads **out-of-process provider plugins** over stdio RPC — no cloud/provider SDKs in the resolver module.

See [plan.md](plan.md) for goals and [docs/design-note.md](docs/design-note.md) for the maintainer-facing writeup.

## Quick demo (sqlite)

```bash
# build
go build -o bin/mamori-resolver ./cmd/mamori-resolver
go build -o bin/mamori-provider-sqlite ./cmd/mamori-provider-sqlite

# seed a tiny DB
go run ./examples/seed-sqlite -db /tmp/mamori-demo.db

# resolve
./bin/mamori-resolver -config examples/providers.sqlite.json \
  'sqlite://config/greeting'
```

`go.mod` at the repo root must stay free of `github.com/xavidop/mamori` and provider SDKs. Provider binaries live under `cmd/mamori-provider-*` / `serve/` and pull those deps themselves.

## Layout

| Path | Role |
| --- | --- |
| `.` (this module) | `Resolver`, RPC client, config — minimal deps |
| `serve/` | Provider-side `Serve(...)` shim (depends on Mamori) |
| `cmd/mamori-resolver` | CLI |
| `cmd/mamori-provider-fake` | In-memory provider for tests |
| `cmd/mamori-provider-sqlite` | Real Mamori sqlite provider as RPC server |
