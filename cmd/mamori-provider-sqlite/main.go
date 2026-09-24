// Command mamori-provider-sqlite serves the Mamori SQLite provider over stdio RPC.
package main

import (
	"fmt"
	"os"

	"github.com/xavidop/mamori/providers/sqlite"
	"github.com/yaronf/mamori-resolver/serve"
)

func main() {
	if err := serve.ServeWith(serve.Options{
		Name: "sqlite",
	}, sqlite.New()); err != nil {
		fmt.Fprintf(os.Stderr, "mamori-provider-sqlite: %v\n", err)
		os.Exit(1)
	}
}
