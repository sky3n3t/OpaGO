package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/wasi_snapshot_preview1"
)

var (
	env, opa   api.Module
	ctx        context.Context
	builtinMap map[int32]topdown.BuiltinFunc
	tCTX       *topdown.BuiltinContext
)

func main() {

	// Choose the context to use for function calls.
	ctx = context.Background()
	tCTX = &topdown.BuiltinContext{
		Context:      ctx,
		Metrics:      metrics.New(),
		Seed:         rand.New(rand.NewSource(0)),
		Time:         ast.NumberTerm(json.Number(strconv.FormatInt(time.Now().UnixNano(), 10))),
		Cancel:       topdown.NewCancel(),
		Runtime:      nil,
		Cache:        make(builtins.Cache),
		Location:     nil,
		Tracers:      nil,
		QueryTracers: nil,
		QueryID:      0,
		ParentID:     0,
	}
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
	env, err = r.NewModuleBuilder("env").
		ExportFunction("opa_abort", opaAbort).
		ExportFunction("opa_builtin0", C0).
		ExportFunction("opa_builtin1", C1).
		ExportFunction("opa_builtin2", C2).
		ExportFunction("opa_builtin3", C3).
		ExportFunction("opa_builtin4", C4).
		ExportFunction("opa_println", opaPrintln).
		ExportMemory("memory", 10).
		Instantiate(ctx, r)
	if err != nil {
		log.Panicln(err)
	}
	opa, err = r.InstantiateModuleFromBinary(ctx, wasm)
	if err != nil {
		log.Panicln(err)
	}
	builtins, err := opa.ExportedFunction("builtins").Call(ctx)
	if err != nil {
		log.Panicln(err)
	}
	builtinsJson, err := opa.ExportedFunction("opa_json_dump").Call(ctx, builtins[0])
	if err != nil {
		log.Panicln(err)
	}
	builtinMap, err = getFuncs(parseJsonBuiltinString(readStr(uint32(builtinsJson[0]), ctx)))
	if err != nil {
		log.Panicln(err)
	}
	//env.Name()
	// Discover 7! is 5040
	ev, err := opa.ExportedFunction("opa_eval_ctx_new").Call(ctx)
	if err != nil {
		log.Panicln(err)
	}
	input := []byte("{\"user\":\"alice\",\"method\":\"get\"}")
	loc, err := opa.ExportedFunction("opa_malloc").Call(ctx, uint64(len(input)))
	if err != nil {
		log.Panicln(err)
	}
	env.Memory().Write(ctx, uint32(loc[0]), input)
	inp, err := opa.ExportedFunction("opa_json_parse").Call(ctx, loc[0], uint64(len(input)))
	if err != nil {
		log.Panicln(err)
	}
	_, err = opa.ExportedFunction("opa_eval_ctx_set_input").Call(ctx, ev[0], inp[0])
	if err != nil {
		log.Panicln(err)
	}
	data := []byte("{\"dataset\":{\"methods\":[\"get\",\"post\",\"put\",\"delete\"],\"users\":{\"alice\":[\"get\"],\"bob\":[\"get\",\"post\"],\"charlie\":[\"get\",\"post\",\"put\"],\"dana\":[\"get\",\"post\",\"put\",\"delete\"]}}}")
	dLoc, err := opa.ExportedFunction("opa_malloc").Call(ctx, uint64(len(data)))
	if err != nil {
		log.Panicln(err)
	}
	env.Memory().Write(ctx, uint32(dLoc[0]), data)
	dat, err := opa.ExportedFunction("opa_json_parse").Call(ctx, dLoc[0], uint64(len(data)))
	if err != nil {
		log.Panicln(err)
	}
	_, err = opa.ExportedFunction("opa_eval_ctx_set_data").Call(ctx, ev[0], dat[0])
	if err != nil {
		log.Panicln(err)
	}
	_, err = opa.ExportedFunction("eval").Call(ctx, ev[0])
	if err != nil {
		log.Panicln(err)
	}
	resLoc, err := opa.ExportedFunction("opa_eval_ctx_get_result").Call(ctx, ev[0])
	if err != nil {
		log.Panicln(err)
	}
	strLoc, err := opa.ExportedFunction("opa_json_dump").Call(ctx, resLoc[0])
	if err != nil {
		log.Panicln(err)
	}
	fmt.Println(readStr(uint32(strLoc[0]), ctx))
	env.ExportedFunction("opa_abort").Call(ctx, strLoc[0])
	fmt.Println(context.Background(), ctx)
}
func readStr(loc uint32, ctx context.Context) string {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b, _ := env.Memory().ReadByte(ctx, loc+index)
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
func opaAbort(ptr int32) {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b, _ := env.Memory().ReadByte(context.Background(), uint32(ptr)+index)
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
	log.Panic("error", out)
}
func Call(args ...int32) int32 {
	var output *ast.Term
	pArgs := []*ast.Term{}
	for _, ter := range args[2:] {
		serialized, err := opa.ExportedFunction("opa_value_dump").Call(ctx, uint64(ter))
		if err != nil {
			log.Panic(err)
		}
		data := readStr(uint32(serialized[0]), ctx)
		pTer, err := ast.ParseTerm(string(data))
		if err != nil {
			log.Panic(err)
		}
		pArgs = append(pArgs, pTer)
	}
	err := builtinMap[args[0]](*tCTX, pArgs, func(t *ast.Term) error {
		output = t
		return nil
	})
	if err != nil {
		if errors.As(err, &topdown.Halt{}) {
			var e *topdown.Error
			if errors.As(err, &e) && e.Code == topdown.CancelErr {
				log.Panic(err)
			}
			log.Panic(err)
		}
		// non-halt errors are treated as undefined ("non-strict eval" is the only
		// mode in wasm), the `output == nil` case below will return NULL
	}
	if output == nil {
		return 0
	}
	outB := []byte(output.String())
	loc, err := opa.ExportedFunction("opa_malloc").Call(ctx, uint64(len(outB)))
	if err != nil {
		log.Panic(err)
	}
	env.Memory().Write(ctx, uint32(loc[0]), outB)
	addr, err := opa.ExportedFunction("opa_value_parse").Call(ctx, uint64(loc[0]), uint64(len(outB)))
	if err != nil {
		log.Panic(err)
	}
	return int32(addr[0])
}
func C0(i, j int32) int32 {
	fmt.Println(builtinMap[i])
	return Call(i, j)
}
func C1(i, j, k int32) int32 {
	fmt.Println(builtinMap[i])
	return Call(i, j, k)
}
func C2(i, j, k, l int32) int32 {
	fmt.Println(builtinMap[i])
	return Call(i, j, k, l)
}
func C3(i, j, k, l, m int32) int32 {
	fmt.Println(builtinMap[i])
	return Call(i, j, k, l, m)
}
func C4(i, j, k, l, m, n int32) int32 {
	fmt.Println(builtinMap[i])
	return Call(i, j, k, l, m, n)
}
func opaPrintln(ptr int32) {

	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b, _ := env.Memory().ReadByte(context.Background(), uint32(ptr)+index)
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
	fmt.Println(out)

}
func getFuncs(ids map[string]int32) (map[int32]topdown.BuiltinFunc, error) {
	out := map[int32]topdown.BuiltinFunc{}
	for name, id := range ids {
		out[id] = topdown.GetBuiltin(name)
		if out[id] == nil {
			return out, fmt.Errorf("no function named %s", name)
		}
	}
	return out, nil
}
func parseJsonBuiltinString(str string) map[string]int32 {
	currKey := ""
	inKey := false
	inVal := false
	currVal := ""
	out := map[string]int32{}
	for _, char := range str {
		switch char {
		case '"':
			inKey = !inKey
		case '{':
		case '}':
			val, _ := strconv.ParseInt(currVal, 10, 32)
			out[currKey] = int32(val)
		case ':':
			inVal = true
		case ',':
			val, _ := strconv.ParseInt(currVal, 10, 32)
			out[currKey] = int32(val)
			inVal = false
			currVal = ""
			currKey = ""
		default:
			if inKey {
				currKey += string(char)
			} else if inVal {
				currVal += string(char)
			}
		}

	}
	return out
}
