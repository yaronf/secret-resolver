# Provider manifest

[`providers.manifest.json`](../providers.manifest.json) lists which Mamori provider packages get a `mamori-provider-*` RPC wrapper. Prefer the script over hand-editing.

## Prerequisite

Mamori must export [`Providers()`](../../mamori.git/registry.go) (registry snapshot). This POC’s `replace` points at a local mamori tree that includes that export. Generated mains blank-import the provider package (its `init` calls `Register`) and then `serve.ServeRegistered()`.

## Add a provider

```bash
./scripts/add-provider github.com/xavidop/mamori/providers/vault
```

That updates the manifest, runs `go generate`, and prints the next `go mod tidy` / `go build` step. Use `-nogen` on `addprovider` to only edit the JSON.

Schemes (including multi-scheme packages like AWS) come from whatever `init` registered — no constructor list in the manifest.

## Manifest fields

| Field | Meaning |
| --- | --- |
| `name` | Short id → binary `cmd/mamori-provider-<name>/`. Defaults to the last segment of `import` when using the script. |
| `import` | Go import path of the Mamori provider module. Blank-imported so `init` registers providers. |

Example:

```json
{
  "name": "aws",
  "import": "github.com/xavidop/mamori/providers/aws"
}
```

Generated `main.go` is not edited by hand (`DO NOT EDIT`). Re-run `go generate .` after any manual manifest change.
