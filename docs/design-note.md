# Design note: Mamori secret resolution via out-of-process providers

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

**This POC:** RPC on a **dedicated Unix socketpair** fd via `exec.Cmd.ExtraFiles` (`MAMORI_RPC_FD`, default 3). **Stdout stays free** for provider logging; stderr is forwarded to the host. Wire encoding is stdlib **gob `net/rpc`** (no length-prefixed framing). Inheritance is the authentication — no shared secret.

**Trust model:** out-of-process providers isolate **dependencies / SDKs**, not adversaries. A replaced binary or hostile plugin is still trusted to resolve secrets for the host. The host uses a minimal child environment (explicit `Provider.Env` for credentials), pins `MAMORI_RPC_FD`, caps inbound RPC reads and value size, and treats unknown error kinds as `unknown`.

**Unix only** (Linux, macOS, …). Mamori’s CI still builds/vets on Windows, so an upstream landing needs a portable path. Candidates (not implemented here):

1. **Loopback TCP + handshake token** (smallest cross-platform change)  
   Host `listen 127.0.0.1:0` → pass `MAMORI_RPC_ADDR` + `MAMORI_RPC_TOKEN` in **env** (never argv — shows up in `ps` / cmdline) → child dials → host **Accept once**, close listener, reject wrong/missing token. Token is required: anything on the machine can race the open port before the child connects. Clear the token from the child’s env after auth if desired.

2. **Unix domain socket path + Windows named pipe** (HashiCorp go-plugin style)  
   Random name, `0600` / same-user ACL; avoids the localhost accept race. Not one stdlib API — platform code (and likely a small Windows helper dep). Preferable long-term if upstream wants to avoid TCP entirely; FIFOs are *not* a substitute (half-duplex).

Socketpair stays the POC default: fewest deps, no filesystem name, no token. Portable transport is a follow-up, not a blocker for proving the dep split.

## Limitations (POC)

- **Unix-only** host↔child transport (`socketpair` + `ExtraFiles`); Windows alternatives documented above, not coded.
- Providers are **not** a security sandbox (dep isolation only).
- Context cancel stops waiting on the host only; the child RPC may continue. When the caller’s `ctx` has a deadline it is sent as `Deadline` so the provider can bound SDK calls — there is no remote cancel RPC in the POC.
- `Close` rejects new Resolves, waits for in-flight ones, then kills children.
- Child env is allowlisted + explicit `Provider.Env` (Provider.Env overrides allowlisted keys; `MAMORI_RPC_FD` is forced last). Cloud file-based ADC still needs `HOME` on the allowlist or an explicit Env path.
- Resolve value size and per-connection gob read budgets are capped.
- Resolve-only; Watch is a later design.

## Evidence from this POC

- Root `go.mod` has **no** `github.com/xavidop/mamori` and no provider SDKs.
- Fake + sqlite providers resolve end-to-end over a dedicated RPC fd.
- Concurrent Resolve works against one child process.
