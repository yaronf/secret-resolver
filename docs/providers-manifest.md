# Provider manifest

[`providers.manifest.json`](../providers.manifest.json) lists which Mamori provider packages get a `mamori-provider-*` RPC wrapper. Prefer adding entries with the script (below) instead of hand-editing.

## Add a provider

```bash
./scripts/add-provider github.com/xavidop/mamori/providers/vault
# or
go run ./internal/addprovider -import github.com/xavidop/mamori/providers/vault
```

Multiple constructors (one process, several schemes — e.g. AWS):

```bash
./scripts/add-provider github.com/xavidop/mamori/providers/aws \
  -new 'NewSecretsManager()' \
  -new 'NewParameterStore()' \
  -new 'NewAppConfig()'
```

That updates the manifest, runs `go generate`, and prints the next `go mod tidy` / `go build` step. Use `-nogen` on `addprovider` to only edit the JSON.

## Manifest fields

Each object under `"providers"`:

| Field | Meaning |
| --- | --- |
| `name` | Short id → binary `cmd/mamori-provider-<name>/`. Defaults to the last segment of `import` when using the script. |
| `import` | Go import path of the existing Mamori provider module (unchanged upstream package). |
| `new` | List of constructor **call expressions** invoked as `prov.<expr>` and passed to `serve.Serve(...)`. Usually `["New()"]`. For packages that register several schemes via separate constructors, list each (as with AWS). |

Example:

```json
{
  "name": "vault",
  "import": "github.com/xavidop/mamori/providers/vault",
  "new": ["New()"]
}
```

Generated `main.go` is not edited by hand (`DO NOT EDIT`). Re-run `go generate .` after any manual manifest change.
