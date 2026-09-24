# mamori-resolver

## Goal

Build a **POC** that can convince the Mamori maintainers of two claims:

1. **A stand-alone resolver is useful** — callers that already have a config stack can resolve Mamori secret URIs without linking Mamori’s full typed-config / watch / reconciliation machinery or any cloud SDKs into their own binary.
2. **Dynamic loading of providers is useful and not too difficult** — at runtime the resolver loads **prebuilt provider binaries** (config → exec → stdio RPC → `Info` schemes), so an install that only needs SQLite never ships or loads AWS, and callers need not **compile** provider source (or its SDKs) into their own module. “Dynamic loading” means **out-of-process provider plugins**, not Go `plugin` `.so` files.

The POC is the argument: working code + a short demo + measurements. It is **not** a production-hardened product.

### Customer value (lead with this in the design note)

**Existing Mamori customers** (already on typed config / watch / reconcile):

- **Smaller, faster builds** — pull only the provider binaries they need at runtime; AWS SDK (etc.) no longer has to enter every app’s `go.mod` / CI cache just because one field might resolve from Secrets Manager.
- **Same URIs and semantics** — `aws-sm://…`, `sqlite://…`, error kinds, `Value` metadata stay familiar; they are not asked to learn a second secret system.
- **Optional adoption** — keep in-process blank-import where that is fine; use out-of-process only where SDK weight, isolation, or selective install matters.
- **Vendor / extension providers** — third parties can ship a `mamori-provider-*` binary that speaks the same RPC without forcing every consumer to compile that vendor’s SDK.

**Future / resolve-only customers** (apps that already have config and only want Mamori for secrets):

- **Strong secret management without a second config framework** — `Resolve(uri) → Value`; leave existing config where it is.
- **Tiny caller dependency** — link a thin resolver, not Mamori core + N cloud SDKs.
- **Install what you use** — download/run only the provider binaries needed; never compile or ship the rest.
- **Clear on-ramp** — start resolve-only; graduate to full Mamori Load/Watch later with the same refs and providers.

The maintainer pitch (architecture, process model, upstream home) comes **after** this value story.

**POC vs eventual proposal:**

| | POC (now) | Eventual upstream proposal |
| --- | --- | --- |
| Provider side | Thin `go generate` wrappers blank-import existing packages — **zero provider source changes** | **Migrate providers into RPC servers** — each `providers/*` ships (or owns) a `mamori-provider-*` entrypoint / `Serve` so the RPC server is first-class, not an external generate hack |
| Why | Prove resolver + protocol + dep split quickly | Make “download binary, don’t compile provider source” the supported model Mamori owns |
| Caller | Still never compiles SDKs into the resolver | Same; plus released provider binaries from Mamori and/or vendors |

### Why this cannot be a long-lived stand-alone project

The resolver depends **100% on Mamori**, and especially on **the provider packages** (APIs, schemes, error semantics, SDKs, release cadence). A forever-external `mamori-resolver` would be a tracking tax: every Mamori provider change, new scheme, or SPI tweak becomes our breakage. The only durable home is **upstream Mamori** (or an officially maintained Mamori subproject).

So the POC’s job is persuasion toward upstream adoption — not founding an independent product. Spike code lives in this repo (`mamori-resolver/`, sibling of `mamori.git`); private remote for collaboration/backup. Long-term maintenance outside Mamori remains a non-starter — the design note must say so.

**Spike layout:** this repository — private GitHub remote (`yaronf/mamori-resolver`) for now; durable home is still upstream Mamori if maintainers adopt it. Reuse the existing `providers/*` modules and `Register` SPI via `replace` / local paths as needed.

## Target

```
URI → select provider process → stdio RPC → existing Mamori provider → Value
```

`mamori-resolver` has **no** AWS/GCP/Azure/Vault/SQLite/etc. SDK dependencies, and otherwise the **minimum possible** dependency graph (stdlib-first: `net/rpc`, framing, process exec). Provider SDKs live only in separately built `mamori-provider-*` executables.

```
                    mamori-resolver
                          │
              ┌───────────┴───────────┐
              │ scheme → process map  │
              │ process manager       │
              │ framed net/rpc client │
              └───────────┬───────────┘
                          │ ExtraFiles fd (socketpair)
          ┌───────────────┼────────────────┐
          ▼               ▼                ▼
 mamori-provider-sqlite  mamori-provider-aws  ...
   (sqlite)                (aws-sm, aws-ps,
                            aws-appconfig)
          │               │
    existing Mamori   existing Mamori
    provider pkgs     provider pkgs
```

**POC provider pair:** **sqlite** (trivial local setup: DB file + `SQLITE_PATH`) as the default live demo; **AWS** optional/nightly to show a heavy SDK stays out of the resolver. Vault (HashiCorp) is a fine later target but is not required for the POC demo.
No ports, sockets, or service discovery.

### Non-goals (POC)

- Production hardening (auto-restart, remote cancel, sophisticated lifecycle)
- Watch / `WatchableProvider`
- `BatchProvider` over RPC
- Reusing Mamori’s HTTP config-server wire protocol (`GET /v1/values/{name}` is named-bindings only)
- Non-Go provider executables (gob / `net/rpc` stays Go↔Go)
- Structured `With*` / injected-client config across the process boundary (env + cloud default credential chains only, e.g. AWS default chain / GCP Application Default Credentials)
- Migrating all Mamori providers to first-class RPC servers **before** the demo (POC uses generate wrappers; migration is the proposal)
- **Maintaining this as a permanent project outside Mamori** (provider coupling makes that untenable)
- Treating generate wrappers as the permanent packaging model (they are a bridge only)

---

## What “done” means for the POC

Maintainers can be shown, in one sitting:

| Claim | Evidence |
| --- | --- |
| Stand-alone resolver is useful | Small Go program (or CLI) resolves `sqlite://…` (and optionally `aws-sm://…`) via `Resolver.Resolve` with **no** provider SDKs in its `go.mod` |
| Dynamic loading is useful | Config lists only the providers needed; sqlite-only run never execs/loads AWS; caller did not compile provider source |
| Dynamic loading is not too hard | Small resolver + shim; POC uses generated blank-import mains; design note shows migration path to providers-as-RPC-servers |

Artifacts to hand over: runnable POC, README demo script, dependency-graph / binary-size notes, and a short design note that **leads with customer value**, then proposes how this lands inside Mamori.

---

## Deliverables (POC-scoped)

1. **`mamori-resolver`** — process manager + framed `net/rpc` + discovery + public `Resolve` / `WithProviders` API (spike module; intended upstream shape).
2. **Provider binaries (POC bridge)** — `go generate` → `mamori-provider-sqlite` (required demo) and optionally `mamori-provider-aws` via blank-import + `ServeRegistered()`; **zero edits** to existing provider packages.
3. **Minimal wire/SPI types in the resolver** — enough for the RPC boundary. Provider children keep importing full `github.com/xavidop/mamori` as today.
4. **Design note** — includes the **migration proposal**: move from generate wrappers to providers shipping as RPC servers upstream.

Upstream adoption of resolver + providers-as-RPC-servers is the desired outcome. A permanent external fork / permanent generate-only packaging layer is rejected.

---

### Phase 0 — Confirm SPI surface (first implementation task)

Inspection of AWS + sqlite (and previously Vault) already shows a thin dependency: production code imports only the root `github.com/xavidop/mamori` package (no reconciler, Load/config, watch adapter, or server). Treat this as **confirmation**, not a blocker.

Symbols the POC must share across the RPC / provider boundary:

| Symbol | Role |
| --- | --- |
| `Provider` | SPI |
| `Ref`, `Value` | request/response (`Value`: Bytes, Version, Sensitive, NotAfter, Metadata) |
| `Register`, registry helpers | blank-import packaging |
| `ParseRef` | shim parses URIs |
| `Err*` / `Kind` / `ErrorKind` | semantic errors on the wire |
| `SelectKey`, `VersionHash` | used inside provider packages |

**POC approach:** provider executables may depend on full `github.com/xavidop/mamori` for the spike (they already do). The **resolver** module must not. Shared wire types can live in the resolver’s `rpc` package duplicated thinly, or a tiny local api module — perfection of the upstream split is not required to prove the claims.

**Spike exit criteria:** resolver `go.mod` has no provider SDKs and only minimal other deps; fake provider + **sqlite** resolves end-to-end locally.

---

### Phase 1 — POC packaging bridge → upstream migration

**POC (no provider source changes):** `go generate` from a manifest produces tiny mains that blank-import existing `providers/*` and call `ServeRegistered()`. Fast proof; does not ask maintainers to touch every provider yet.

**Why that alone is not the end state:** Dynamic loading is most valuable when users **do not compile provider source** — they install/run **prebuilt** `mamori-provider-*` binaries. A forever-external generate layer that re-wraps Mamori packages is awkward and still couples us to tracking every provider module. The durable story is **Mamori and/or vendors releasing** those binaries (first-party providers from Mamori CI; third-party providers from the vendor that owns them).

**Eventual proposal (design note + maintainer ask):** **migrate providers into RPC servers** —

- Shared `serve.ServeRegistered()` lives in this POC (uses `mamori.Providers()`).
- Each provider module gains a first-class server entrypoint (e.g. `providers/aws/cmd/mamori-provider-aws`, or a one-line `main` built in that provider’s CI) — part of that provider’s tree/release, whether Mamori-owned or vendor-owned; not an external generate hack.
- In-process `Register` + blank-import remains for apps that still want to link providers; out-of-process becomes the path for “deps stay out of my binary / I only need sqlite.”
- Resolver ships from Mamori; provider binaries ship from Mamori (built-ins) and from vendors (extensions) speaking the same RPC.

POC generate wrappers demonstrate the wire protocol and dep split; migration makes providers the RPC servers of record.

---

### Phase 2 — Define `mamori-resolver`

```
mamori-resolver/
    resolver.go
    registry.go
    process.go
    rpc/
        protocol.go
        codec.go      # framed ReadWriteCloser over stdio
        client.go
        server.go
    cmd/
        mamori-resolver/
    internal/
        ...
```

Public API:

```
type Resolver struct { ... }

func New(opts ...Option) (*Resolver, error)

func (r *Resolver) Resolve(
    ctx context.Context,
    uri string,
) (Value, error)

func (r *Resolver) Close() error
```

Return full `Value`, not bare `[]byte` — providers already set Version, Sensitive, NotAfter, Metadata (e.g. AWS SM sensitivity; Vault leases when we add that provider later).

Do **not** pull Mamori’s typed-config / watch / reconciliation machinery into this project.

---

### Phase 3 — Tiny RPC protocol

Stdlib `net/rpc` + gob on a **dedicated socketpair** fd (`ExtraFiles` / `MAMORI_RPC_FD=3`). Stdout stays free for logging.

Protocol version applies to request/response shapes (`ProtocolVersion = 1` from day one). `ProviderVersion` is the provider binary/module version (distinct from `Value.Version`, which is a secret revision).

```
type InfoRequest struct{}

type InfoResponse struct {
    ProtocolVersion uint32
    ProviderName    string
    ProviderVersion string   // provider binary/module version (e.g. semver / release tag)
    Schemes         []string // one binary may expose many (e.g. aws-sm, aws-ps, aws-appconfig)
}

type ResolveRequest struct {
    URI      string
    Deadline *time.Time  // optional; child bounds SDK calls when set
}

type ResolveResponse struct {
    Bytes     []byte
    Version   string
    Sensitive bool
    NotAfter  time.Time
    Metadata  map[string]string
}

type RPCError struct {
    Kind    string  // mamori Kind, e.g. "not_found"
    Message string  // never include secret material
}
```

Methods:

```
Mamori.Info
Mamori.Resolve
```

Errors: do not transmit only `error.Error()`. Map to/from Mamori sentinels via `Kind` so callers can `errors.Is(err, ErrNotFound)` etc.

**Context:** classic `net/rpc` does not propagate `context.Context`. Resolver stops waiting when `ctx` expires (`Client.Go`); the remote call may continue. `Deadline` on the request lets the child apply a bound. Explicit cancel RPCs are out of scope for the POC.

Children remain Go binaries (gob). Non-Go providers are a non-goal.

---

### Phase 4 — Generic provider-side shim

Single entry point for generated mains:

```
func ServeRegistered() error
```

(Blank-import `init()` + `Register` is the packaging model — do not require each main to pass `Serve(providers...)`.)

The shim:

1. serves registered providers over framed stdio RPC;
2. exposes schemes via `Info` (union of all `Register`ed schemes in-process);
3. parses URIs with `ParseRef`;
4. dispatches `Resolve` to the matching provider;
5. translates `Value` / error kinds to wire types;
6. on shutdown, calls `Close()` on providers that implement `io.Closer`;
7. serves RPC on the inherited dedicated fd (stdout free).

Provider executables contain essentially **zero** hand-written RPC code.

---

### Phase 5 — Provider executables (POC: generate; proposal: migrate)

**POC:** manifest + `go generate` → wrapper mains (see Phase 1). Example generated `main.go`:

```
package main

import (
    _ "github.com/xavidop/mamori/providers/aws"
    "github.com/.../mamori-resolver/rpc"
)

func main() {
    if err := rpc.ServeRegistered(); err != nil {
        os.Exit(1)
    }
}
```

Manifest is package-level (one AWS package → three schemes: `aws-sm`, `aws-ps`, `aws-appconfig`; sqlite → `sqlite`).

**Upstream migration (proposed, not POC work):** same binary shape, but `main` / `Serve` lives under each `providers/*` (or Mamori-owned `cmd/mamori-provider-*` built from that package), released as artifacts. Generate wrappers go away once providers are RPC servers.

Resulting dependency split (POC and end state):

```
mamori-resolver            ~minimal deps (no provider SDKs; stdlib-first)

mamori-provider-sqlite     └ modernc.org/sqlite (etc.); scheme: sqlite
mamori-provider-aws        └ AWS SDK; schemes: aws-sm, aws-ps, aws-appconfig
```
---

### Phase 6 — Configuration and discovery

**POC:** configure only via API — `resolver.WithProviders(resolver.Provider{Command, Args, Env}...)`. The demo CLI maps flags (`-provider`, `-env`) onto that API. **No** separate resolver JSON/YAML schema.

**Upstream:** if Mamori grows a file format for out-of-process providers, it belongs in **Mamori’s existing YAML config**, not a parallel format in this module.

**POC credentials = process environment + each cloud SDK’s default credential chain** (env vars, shared config files, instance/workload identity, in-cluster config). No RPC for `WithToken` / `WithClient`. Document that limitation.

Startup:

```
WithProviders / CLI flags
  → exec each command (inherit/override env)
  → establish framed stdio RPC
  → Mamori.Info()
  → register every returned scheme → that process
```

Config does **not** list schemes; the executable is authoritative. Duplicate schemes across processes are a **startup error**.

---

### Phase 7 — Process lifecycle

Required before calling the POC demo-ready:

- start + detect startup failure / protocol mismatch
- detect unexpected exit mid-flight
- `Resolver.Close()` → RPC shutdown → child `io.Closer` (if any) → process exit
- stderr forwarding to host logs (redact secrets)
- concurrent Resolve against one client/codec
- honor caller `ctx` for wait abandonment; pass `Deadline` when present

**No automatic restart** of crashed providers — surface a clear resolver error.

---

### Phase 8 — Watch as deliberate v2

Mamori’s `WatchableProvider` is channel-based; `net/rpc` is not naturally streaming. The POC is **Resolve only** (fits “secrets into an existing config stack”).

Possible later shape (not designed now):

```
WatchStart / WatchNext / WatchStop
```

---

### Phase 9 — Testing

Resolver suite (fake provider executable → RPC → `Resolver`):

- concurrent Resolve
- binary / large values
- full `Value` metadata round-trip (Version, Sensitive, NotAfter, Metadata)
- error kinds (`not_found`, permission, unavailable, …)
- provider panic / unexpected exit
- malformed startup / protocol-version mismatch
- duplicate schemes
- stderr isolation (no secret bytes in logs/errors)
- context timeout / deadline field
- clean shutdown + Closer

Hermetic CI: fake provider + **sqlite** (temp DB file). Real AWS optional/nightly, not required green path.

Reuse Mamori conformance *philosophy* (semantics), not necessarily by linking heavy `providertest` into the resolver module if that pulls unwanted deps.

---

### Phase 10 — Demo pack for maintainers

Build:

```
mamori-resolver
mamori-provider-sqlite
mamori-provider-aws      # optional for heavy-SDK demo
```

Ship with the POC:

1. **Demo script** — resolve a `sqlite://…` URI from a local DB file; optionally show an AWS URI; show sqlite-only config never starts the AWS process.
2. **Measurements** — resolver `go mod graph` (no provider SDKs; call out every direct dep), binary sizes, cold start, first/subsequent Resolve latency.
3. **Design note** — **first: customer value** (...). Then: upstream home, generate bridge → providers-as-RPC-servers, dedicated RPC fd, Resolve-only.

**POC success = maintainers can evaluate both claims from (1)–(3) and see a credible migration from generate wrappers to providers-as-RPC-servers upstream.** Production readiness and a permanent external repo are out of scope.

---

## Work order

1. Phase 0 spike (resolver graph clean + one end-to-end resolve)
2. Phases 3–4 protocol + shim + fake-provider tests
3. Phases 2, 6–7 resolver host + lifecycle
4. Phase 5 package sqlite (+ optional AWS)
5. Phase 9 tests (enough to trust the demo)
6. Phase 10 demo pack + design note
7. Design note: migration path — providers as RPC servers (Mamori-owned binaries)
