# Design note: stand-alone resolver + out-of-process providers

## Customer value (first)

### Existing Mamori customers

- **Smaller, faster builds** — only the provider binaries you need at runtime; AWS SDK (etc.) need not enter every app’s `go.mod`.
- **Same URIs and semantics** — `sqlite://…`, `aws-sm://…`, error kinds, `Value` metadata stay familiar.
- **Optional adoption** — keep in-process blank-import where fine; use out-of-process where SDK weight or selective install matters.
- **Vendor providers** — third parties can ship a `mamori-provider-*` binary speaking the same RPC without forcing consumers to compile that vendor’s SDK.

### Resolve-only newcomers (MCP, CLIs, one-shot fetch)

- Mamori’s **provider ecosystem without** typed-config / watch / reconcile.
- Tiny caller dependency: link the resolver, not Mamori core + N SDKs.
- Install what you use; graduate to full Mamori later with the same refs.

## Why this belongs in Mamori

The resolver depends 100% on Mamori providers (schemes, SPI, releases). A forever-external project is a tracking tax. Durable home: upstream Mamori (or an official subproject). Mamori and vendors release prebuilt provider binaries.

## POC vs migration

| Now (this POC) | Eventual proposal |
| --- | --- |
| `go generate` / thin `main` calling `serve.Serve(sqlite.New())` — **zero edits** to provider packages | Providers become first-class RPC servers under `providers/*/cmd/…`; Mamori CI releases binaries |
| Proves wire protocol + dep split | Users download binaries; need not compile provider source |

`mamori.Register`’s lookup is unexported today, so the POC passes provider instances into `Serve` instead of blank-import + `ServeRegistered`. Upstream may export `ProviderFor` or keep explicit `Serve(New())`.

## Sharp edges

- stdio + gob `net/rpc` needs a careful codec; stderr is diagnostics only.
- Context cancel stops waiting locally; optional `Deadline` on the request bounds the child.
- Credentials: env + cloud default chains only in the POC.
- Resolve-only; Watch is a later design.

## Evidence from this spike

- Root `go.mod` has **no** `github.com/xavidop/mamori` and no provider SDKs.
- Fake + sqlite providers resolve end-to-end over stdio RPC.
- Concurrent Resolve works against one child process.
