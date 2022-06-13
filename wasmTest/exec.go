package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"
)

func main() {
	// Choose the context to use for function calls.
	ctx := context.Background()

	// Read a WebAssembly binary containing an exported "fac" function.
	// * Ex. (func (export "fac") (param i64) (result i64) ...
	wasm, err := os.ReadFile("./wasm.wasm")
	if err != nil {
		log.Panicln(err)
	}
	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntime()
	defer r.Close(ctx) // This closes everything this Runtime created.

	// Instantiate the module and return its exported functions
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		log.Panicln(err)
	}
	module, err := r.InstantiateModuleFromBinary(ctx, wasm)
	if err != nil {
		log.Panicln(err)
	}

	// Discover 7! is 5040
	fmt.Println(module.ExportedFunction("factorial"))
	fmt.Println(module.ExportedFunction("factorial").Call(ctx, 33))
}
