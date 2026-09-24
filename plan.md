# mamori-resolver

## Goal

Build a **POC** that can convince the Mamori maintainers of two claims:

1. **A stand-alone resolver is useful** — callers (e.g. MCP secret resolution) can resolve Mamori URIs without linking Mamori’s full typed-config / watch / reconciliation stack or any cloud SDKs into their own binary.
2. **Dynamic loading of providers is useful and not too difficult** — at runtime the resolver loads **prebuilt provider binaries** (config → exec → stdio RPC → `Info` schemes), so an install that only needs Vault never ships or loads AWS, and callers need not **compile** provider source (or its SDKs) into their own module. “Dynamic loading” means **out-of-process provider plugins**, not Go `plugin` `.so` files.

The POC is the argument: working code + a short demo + measurements. It is **not** a production-hardened product.

**POC vs eventual proposal:**

| | POC (now) | Eventual upstream proposal |
| --- | --- | --- |
| Provider side | Thin `go generate` wrappers blank-import existing packages — **zero provider source changes** | **Migrate providers into RPC servers** — each `providers/*` ships (or owns) a `mamori-provider-*` entrypoint / `Serve` so the RPC server is first-class, not an external generate hack |
| Why | Prove resolver + protocol + dep split quickly | Make “download binary, don’t compile provider source” the supported model Mamori owns |
| Caller | Still never compiles SDKs into the resolver | Same; plus released provider binaries from Mamori and/or vendors |

### Why this cannot be a long-lived stand-alone project

The resolver depends **100% on Mamori**, and especially on **the provider packages** (APIs, schemes, error semantics, SDKs, release cadence). A forever-external `mamori-resolver` would be a tracking tax: every Mamori provider change, new scheme, or SPI tweak becomes our breakage. The only durable home is **upstream Mamori** (or an officially maintained Mamori subproject).

So the POC’s job is persuasion toward upstream adoption — not founding an independent product. Spike code lives in a **local git repo** beside `mamori.git` for now (no remote required); the design note must say clearly that long-term maintenance outside Mamori is a non-starter.

**Spike layout:** this repository (`mamori-resolver/`, sibling of `mamori.git`) — local `git` only for now, no remote until/unless maintainers want it upstream. Reuse the existing `providers/*` modules and `Register` SPI via `replace` / local paths as needed.

## Target

```
URI → select provider process → stdio RPC → existing Mamori provider → Value
```

`mamori-resolver` has **no** AWS/GCP/Azure/Vault/etc. SDK dependencies, and otherwise the **minimum possible** dependency graph (stdlib-first: `net/rpc`, framing, process exec, small config parse). Provider SDKs live only in separately built `mamori-provider-*` executables.

```
                    mamori-resolver
                          │
              ┌───────────┴───────────┐
              │ scheme → process map  │
              │ process manager       │
              │ framed net/rpc client │
              └───────────┬───────────┘
                          │ stdin/stdout (framed)
          ┌───────────────┼────────────────┐
          ▼               ▼                ▼
 mamori-provider-aws  mamori-provider-vault  ...
   (aws-sm, aws-ps,        (vault)
    aws-appconfig)
          │               │
    existing Mamori   existing Mamori
    provider pkgs     provider pkgs
```

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
| Stand-alone resolver is useful | Small Go program (or CLI) resolves `vault://…` / `aws-sm://…` via `Resolver.Resolve` with **no** cloud SDKs in its `go.mod` |
| Dynamic loading is useful | Config lists only the providers needed; Vault-only run never execs/loads AWS; caller did not compile provider source |
| Dynamic loading is not too hard | Small resolver + shim; POC uses generated blank-import mains; design note shows migration path to providers-as-RPC-servers |

Artifacts to hand over: runnable POC, README demo script, dependency-graph / binary-size notes, and a short design note proposing **how this lands inside Mamori** (not how we maintain it forever outside).

---

## Deliverables (POC-scoped)

1. **`mamori-resolver`** — process manager + framed `net/rpc` + config/discovery + public `Resolve` API (spike module; intended upstream shape).
2. **Provider binaries (POC bridge)** — `go generate` → `mamori-provider-aws` / `mamori-provider-vault` via blank-import + `ServeRegistered()`; **zero edits** to existing provider packages.
3. **Minimal wire/SPI types in the resolver** — enough for the RPC boundary. Provider children keep importing full `github.com/xavidop/mamori` as today.
4. **Design note** — includes the **migration proposal**: move from generate wrappers to providers shipping as RPC servers upstream.

Upstream adoption of resolver + providers-as-RPC-servers is the desired outcome. A permanent external fork / permanent generate-only packaging layer is rejected.

---

### Phase 0 — Confirm SPI surface (first implementation task)

Inspection of AWS + Vault already shows a thin dependency: production code imports only the root `github.com/xavidop/mamori` package (no reconciler, Load/config, watch adapter, or server). Treat this as **confirmation**, not a blocker.

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

**Spike exit criteria:** resolver `go.mod` has no cloud SDKs and only minimal other deps; fake provider + one real provider (Vault or AWS) resolves end-to-end.

---

### Phase 1 — POC packaging bridge → upstream migration

**POC (no provider source changes):** `go generate` from a manifest produces tiny mains that blank-import existing `providers/*` and call `ServeRegistered()`. Fast proof; does not ask maintainers to touch every provider yet.

**Why that alone is not the end state:** Dynamic loading is most valuable when users **do not compile provider source** — they install/run **prebuilt** `mamori-provider-*` binaries. A forever-external generate layer that re-wraps Mamori packages is awkward and still couples us to tracking every provider module. The durable story is **Mamori and/or vendors releasing** those binaries (first-party providers from Mamori CI; third-party providers from the vendor that owns them).

**Eventual proposal (design note + maintainer ask):** **migrate providers into RPC servers** —

- Shared `rpc.ServeRegistered()` (or `Serve(providers...)`) lives in Mamori.
- Each provider module gains a first-class server entrypoint (e.g. `providers/aws/cmd/mamori-provider-aws`, or a one-line `main` built in that provider’s CI) — part of that provider’s tree/release, whether Mamori-owned or vendor-owned; not an external generate hack.
- In-process `Register` + blank-import remains for apps that still want to link providers; out-of-process becomes the path for “deps stay out of my binary / I only need Vault.”
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

Return full `Value`, not bare `[]byte` — providers already set Version, Sensitive, NotAfter, Metadata (e.g. Vault leases, AWS SM sensitivity).

Do **not** pull Mamori’s typed-config / watch / reconciliation machinery into this project.

---

### Phase 3 — Tiny RPC protocol

Stdlib `net/rpc` + gob over a **framed** full-duplex codec on the child’s stdin/stdout. Raw pipes are not enough; `net/rpc` needs an `io.ReadWriteCloser` with concurrent-safe framing (stdlib `rpc.Client` is concurrent-safe; the custom codec must be too — Mamori requires concurrent-safe `Resolve`, and tests will stress this).

stderr is **diagnostics only** — never mix logs onto stdout.

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
7. reserves stderr for diagnostics.

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

Manifest is package-level (one AWS package → three schemes: `aws-sm`, `aws-ps`, `aws-appconfig`).

**Upstream migration (proposed, not POC work):** same binary shape, but `main` / `Serve` lives under each `providers/*` (or Mamori-owned `cmd/mamori-provider-*` built from that package), released as artifacts. Generate wrappers go away once providers are RPC servers.

Resulting dependency split (POC and end state):

```
mamori-resolver          ~minimal deps (no cloud SDKs; stdlib-first)

mamori-provider-aws      └ AWS SDK; schemes: aws-sm, aws-ps, aws-appconfig
mamori-provider-vault    └ Vault SDK; scheme: vault
```
---

### Phase 6 — Configuration and discovery

```
providers:
  - command: mamori-provider-aws
    env:
      AWS_REGION: eu-west-1
  - command: mamori-provider-vault
    env:
      VAULT_ADDR: https://vault.example:8200
      VAULT_TOKEN: ...
```

**POC credentials = process environment + each cloud SDK’s default credential chain** (env vars, shared config files, instance/workload identity, in-cluster config). No RPC for `WithToken` / `WithClient`. Document that limitation.

Startup:

```
read config
  → exec each command (inherit/override env)
  → establish framed stdio RPC
  → Mamori.Info()
  → register every returned scheme → that process
```

YAML does **not** list schemes; the executable is authoritative. Duplicate schemes across processes are a **startup error**.

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

Mamori’s `WatchableProvider` is channel-based; `net/rpc` is not naturally streaming. The POC is **Resolve only** (fits MCP secret resolution).

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

Hermetic CI: fake provider + Vault (or Vault-like fake). Real AWS optional/nightly, not required green path.

Reuse Mamori conformance *philosophy* (semantics), not necessarily by linking heavy `providertest` into the resolver module if that pulls unwanted deps.

---

### Phase 10 — Demo pack for maintainers

Build:

```
mamori-resolver
mamori-provider-aws
mamori-provider-vault
```

Ship with the POC:

1. **Demo script** — resolve one Vault URI and one AWS URI; show Vault-only config never starts the AWS process.
2. **Measurements** — resolver `go mod graph` (no cloud SDKs; call out every direct dep), binary sizes, cold start, first/subsequent Resolve latency.
3. **Design note** — why resolver + out-of-process providers belong **in Mamori**; why an external long-lived project is a bad fit; **POC bridge** (`go generate` wrappers, zero provider edits) vs **eventual migration** (providers become RPC servers; Mamori and vendors release prebuilt binaries so users need not compile provider source); sharp edges (stdio framing, errors, credentials).

**POC success = maintainers can evaluate both claims from (1)–(3) and see a credible migration from generate wrappers to providers-as-RPC-servers upstream.** Production readiness and a permanent external repo are out of scope.

---

## Work order

1. Phase 0 spike (resolver graph clean + one end-to-end resolve)
2. Phases 3–4 protocol + shim + fake-provider tests
3. Phases 2, 6–7 resolver host + lifecycle
4. Phase 5 package AWS + Vault
5. Phase 9 tests (enough to trust the demo)
6. Phase 10 demo pack + design note
7. Design note: migration path — providers as RPC servers (Mamori-owned binaries)
