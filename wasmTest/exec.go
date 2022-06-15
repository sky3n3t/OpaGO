package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"unsafe"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"
)

func main() {

	// Choose the context to use for function calls.
	ctx := context.Background()
	// Read a WebAssembly binary containing an exported "fac" function.
	// * Ex. (func (export "fac") (param i64) (result i64) ...
	wasm, err := os.ReadFile("./policy.wasm")
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
	env, err := r.NewModuleBuilder("env").
		ExportFunction("opa_abort", opaAbort).
		ExportFunction("opa_builtin0", func(i, j int32) int32 { return i }).
		ExportFunction("opa_builtin1", func(i, j, k int32) int32 { return i }).
		ExportFunction("opa_builtin2", func(i, j, k, l int32) int32 { return i }).
		ExportFunction("opa_builtin3", func(i, j, k, l, m int32) int32 { return i }).
		ExportFunction("opa_builtin4", func(i, j, k, l, m, n int32) int32 { return i }).
		ExportFunction("opa_println", opaPrintln).
		ExportMemory("memory", 2).
		Instantiate(ctx, r)
	if err != nil {
		log.Panicln(err)
	}
	module, err := r.InstantiateModuleFromBinary(ctx, wasm)
	if err != nil {
		log.Panicln(err)
	}
	//env.Name()
	// Discover 7! is 5040
	ev, err := module.ExportedFunction("opa_eval_ctx_new").Call(ctx)
	if err != nil {
		log.Panicln(err)
	}
	input := []byte("{\"user\":\"alice\",\"method\":\"post\"}")
	loc, err := module.ExportedFunction("opa_malloc").Call(ctx, uint64(len(input)))
	if err != nil {
		log.Panicln(err)
	}
	env.Memory().Write(ctx, uint32(loc[0]), input)
	inp, err := module.ExportedFunction("opa_json_parse").Call(ctx, loc[0], uint64(len(input)))
	if err != nil {
		log.Panicln(err)
	}
	_, err = module.ExportedFunction("opa_eval_ctx_set_input").Call(ctx, ev[0], inp[0])
	if err != nil {
		log.Panicln(err)
	}
	data := []byte("{\"dataset\":{\"methods\":[\"get\",\"post\",\"put\",\"delete\"],\"users\":{\"alice\":[\"get\"],\"bob\":[\"get\",\"post\"],\"charlie\":[\"get\",\"post\",\"put\"],\"dana\":[\"get\",\"post\",\"put\",\"delete\"]}}}")
	dLoc, err := module.ExportedFunction("opa_malloc").Call(ctx, uint64(len(data)))
	if err != nil {
		log.Panicln(err)
	}
	env.Memory().Write(ctx, uint32(dLoc[0]), data)
	dat, err := module.ExportedFunction("opa_json_parse").Call(ctx, dLoc[0], uint64(len(data)))
	if err != nil {
		log.Panicln(err)
	}
	_, err = module.ExportedFunction("opa_eval_ctx_set_data").Call(ctx, ev[0], dat[0])
	if err != nil {
		log.Panicln(err)
	}
	_, err = module.ExportedFunction("eval").Call(ctx, ev[0])
	if err != nil {
		log.Panicln(err)
	}
	resLoc, err := module.ExportedFunction("opa_eval_ctx_get_result").Call(ctx, ev[0])
	if err != nil {
		log.Panicln(err)
	}
	strLoc, err := module.ExportedFunction("opa_json_dump").Call(ctx, resLoc[0])
	if err != nil {
		log.Panicln(err)
	}
	fmt.Println(readStr(env.Memory(), uint32(strLoc[0]), ctx))
	env.ExportedFunction("opa_println").Call(ctx, strLoc[0])
	fmt.Println(context.Background(), ctx)
}
func readStr(mem api.Memory, loc uint32, ctx context.Context) string {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b, _ := mem.ReadByte(ctx, loc+index)
		if b == 0b0 {
			ok = false
		} else {
			bytes = append(bytes, b)
		}
		index++
	}
	out := ""
	for _, b := range bytes {
		out += string(b)
	}
	return out
}
func opaAbort(i int32) {
	panic(string(i))
}
func opaPrintln(ptr int32) {

	fmt.Println(*(*string)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(ptr), Len: 0, Cap: 0})))

}
