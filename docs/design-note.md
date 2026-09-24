# Design note: stand-alone resolver + out-of-process providers

## Customer value (first)

### Existing Mamori customers

- **Smaller, faster builds** — only the provider binaries you need at runtime; AWS SDK (etc.) need not enter every app’s `go.mod`.
- **Same URIs and semantics** — `sqlite://…`, `aws-sm://…`, error kinds, `Value` metadata stay familiar.
- **Optional adoption** — keep in-process blank-import where fine; use out-of-process where SDK weight or selective install matters.
- **Vendor providers** — third parties can ship a `mamori-provider-*` binary speaking the same RPC without forcing consumers to compile that vendor’s SDK.

### Resolve-only newcomers (already have config; want secrets)

Projects that already own a config subsystem and only need **strong secret management** — Mamori’s providers and URI semantics without adopting typed-config / watch / reconcile.

- Resolve Mamori URIs (`aws-sm://…`, `vault://…`, …) into values; leave the rest of config where it is.
- Tiny caller dependency: link the resolver, not Mamori core + N SDKs.
- Install only the provider binaries you need.
- Optional later on-ramp to full Mamori Load/Watch with the same refs.

## Why this belongs in Mamori

The resolver depends 100% on Mamori providers (schemes, SPI, releases). A forever-external project is a tracking tax. Durable home: upstream Mamori (or an official subproject). Mamori and vendors release prebuilt provider binaries.

## POC vs migration

| Now (this POC) | Eventual proposal |
| --- | --- |
| Blank-import + `serve.ServeRegistered()` via `go generate` — **zero edits** to provider packages (needs mamori `Providers()` export) | Same packaging owned upstream; Mamori CI releases binaries |
| Proves wire protocol + dep split | Users download binaries; need not compile provider source |

`mamori.Providers()` is the small upstream-facing API add (snapshot of the Register registry). Without it, blank-import cannot feed `Serve`. This POC’s `replace` uses a local mamori tree that includes the export.

## Transport

RPC runs on a **dedicated Unix socketpair** fd passed via `exec.Cmd.ExtraFiles` (`MAMORI_RPC_FD`, default 3). **Stdout stays free** for normal provider logging; stderr is forwarded to the host. No stdin/stdout framing.

## Limitations (POC)

- Context cancel stops waiting locally; optional `Deadline` on the request bounds the child.
- Credentials: env + cloud default chains only in the POC.
- Resolve-only; Watch is a later design.

## Evidence from this POC

- Root `go.mod` has **no** `github.com/xavidop/mamori` and no provider SDKs.
- Fake + sqlite providers resolve end-to-end over a dedicated RPC fd.
- Concurrent Resolve works against one child process.
