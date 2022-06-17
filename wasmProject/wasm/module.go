package wasm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type moduleOpts struct {
	name       string
	policy     []byte
	ctx        context.Context
	MinMemSize int
	vm         *VM
}

//wrapper for wazero module
type Module struct {
	module, env api.Module
	name        string
	ctx         context.Context
	tCTX        *topdown.BuiltinContext
	vm          *VM
	builtinT    map[int32]topdown.BuiltinFunc
	entrypointT map[string]int32
}

func (m *Module) newEnv(ctx context.Context, r wazero.Runtime, MemSize int) (api.Module, error) {

	return r.NewModuleBuilder("env").
		ExportFunction("opa_abort", m.opaAbort).
		ExportFunction("opa_builtin0", m.C0).
		ExportFunction("opa_builtin1", m.C1).
		ExportFunction("opa_builtin2", m.C2).
		ExportFunction("opa_builtin3", m.C3).
		ExportFunction("opa_builtin4", m.C4).
		ExportFunction("opa_println", m.opaPrintln).
		ExportMemory("memory", uint32(MemSize)).
		Instantiate(ctx, r)

}
func (m *Module) GetEntrypoints() map[string]int32 {
	eLoc := m.entrypoints()
	return parseJsonString(m.fromRegoJSON(eLoc))
}
func (m *Module) opaAbort(ptr int32) {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b := m.readMemByte(uint32(ptr) + index)
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
func (m *Module) Call(args ...int32) int32 {
	var output *ast.Term
	pArgs := []*ast.Term{}
	for _, ter := range args[2:] {
		serialized, err := m.module.ExportedFunction("opa_value_dump").Call(m.ctx, uint64(ter))
		if err != nil {
			log.Panic(err)
		}
		data := m.readStr(uint32(serialized[0]))
		pTer, err := ast.ParseTerm(string(data))
		if err != nil {
			log.Panic(err)
		}
		pArgs = append(pArgs, pTer)
	}
	err := m.builtinT[args[0]](*m.tCTX, pArgs, func(t *ast.Term) error {
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
	loc := m.writeMem(outB)
	addr, err := m.module.ExportedFunction("opa_value_parse").Call(m.ctx, uint64(loc), uint64(len(outB)))
	if err != nil {
		log.Panic(err)
	}
	return int32(addr[0])
}
func (m *Module) C0(i, j int32) int32 {
	return m.Call(i, j)
}
func (m *Module) C1(i, j, k int32) int32 {
	return m.Call(i, j, k)
}
func (m *Module) C2(i, j, k, l int32) int32 {
	return m.Call(i, j, k, l)
}
func (m *Module) C3(i, j, k, l, n int32) int32 {
	return m.Call(i, j, k, l, n)
}
func (m *Module) C4(i, j, k, l, n, o int32) int32 {
	return m.Call(i, j, k, l, n, o)
}
func (m *Module) opaPrintln(ptr int32) {

	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b := m.readMemByte(uint32(ptr) + index)
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
func newModule(opts moduleOpts, r wazero.Runtime) Module {
	m := Module{}
	m.name = opts.name
	m.vm = opts.vm
	m.tCTX = &topdown.BuiltinContext{
		Context:      opts.ctx,
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
	m.ctx = opts.ctx
	var err error

	m.env, err = m.newEnv(opts.ctx, r, opts.MinMemSize)

	if err != nil {
		log.Panic(err)
	}
	m.module, err = r.InstantiateModuleFromBinary(opts.ctx, opts.policy)
	if err != nil {
		log.Panic(err)
	}
	m.builtinT = newBuiltinTable(m)

	return m
}

// memory accessors
func (m *Module) readMem(offset, length uint32) []byte {
	data, overflow := m.env.Memory().Read(m.ctx, offset, length)
	if !overflow {
		log.Panic("memory index out of range")
	}
	return data
}
func (m *Module) readMemByte(offset uint32) byte {
	data, overflow := m.env.Memory().ReadByte(m.ctx, offset)
	if !overflow {
		log.Panic("memory index out of range")
	}
	return data
}
func (m *Module) writeMem(data []byte) uint32 {

	addr := m.malloc(uint32(len(data)))

	overflow := m.env.Memory().Write(m.ctx, addr, data)
	if !overflow {
		log.Panic("memory index out of range")
	}
	return addr
}
func (m *Module) readStr(loc uint32) string {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b := m.readMemByte(loc + index)
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
func (m *Module) fromRegoJSON(addr uint32) string {
	dump_addr := m.json_dump(addr)

	str := m.readStr(dump_addr)
	return str
}

func (m *Module) wasm_abi_version() int32 {
	return int32(m.module.ExportedGlobal("opa_wasm_abi_version").Get(m.ctx))
}
func (m *Module) wasm_abi_minor_version() int32 {
	return int32(m.module.ExportedGlobal("opa_wasm_abi_minor_version").Get(m.ctx))
}
func (m *Module) eval(ctx_addr uint32) {
	_, err := m.module.ExportedFunction("eval").Call(m.ctx, uint64(ctx_addr))
	if err != nil {
		log.Panic(err)
	}
}
func (m *Module) builtins() uint32 {
	addr, err := m.module.ExportedFunction("builtins").Call(m.ctx)
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) entrypoints() uint32 {
	addr, err := m.module.ExportedFunction("entrypoints").Call(m.ctx)
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) eval_ctx_new() uint32 {
	addr, err := m.module.ExportedFunction("opa_eval_ctx_new").Call(m.ctx)
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) eval_ctx_set_input(ctx_addr, value_addr uint32) {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_input").Call(m.ctx, uint64(ctx_addr), uint64(value_addr))
	if err != nil {
		log.Panic(err)
	}
}
func (m *Module) eval_ctx_set_data(ctx_addr, value_addr uint32) {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_data").Call(m.ctx, uint64(ctx_addr), uint64(value_addr))
	if err != nil {
		log.Panic(err)
	}
}
func (m *Module) eval_ctx_set_entrypoint(ctx_addr, entrypoint_id uint32) {
	_, err := m.module.ExportedFunction("opa_eval_ctx_set_data").Call(m.ctx, uint64(ctx_addr), uint64(entrypoint_id))
	if err != nil {
		log.Panic(err)
	}
}
func (m *Module) eval_ctx_get_result(ctx_addr uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_eval_ctx_get_result").Call(m.ctx, uint64(ctx_addr))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) malloc(size uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_malloc").Call(m.ctx, uint64(size))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) free(addr uint32) {
	_, err := m.module.ExportedFunction("opa_free").Call(m.ctx, uint64(addr))
	if err != nil {
		log.Panic(err)
	}
}
func (m *Module) json_parse(str_addr, size uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_json_parse").Call(m.ctx, uint64(str_addr), uint64(size))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) value_parse(str_addr, size uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_value_parse").Call(m.ctx, uint64(str_addr), uint64(size))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) json_dump(value_addr uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_json_dump").Call(m.ctx, uint64(value_addr))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) value_dump(value_addr uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_value_dump").Call(m.ctx, uint64(value_addr))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) heap_ptr_set(addr uint32) {
	_, err := m.module.ExportedFunction("opa_heap_ptr_set").Call(m.ctx, uint64(addr))
	if err != nil {
		log.Panic(err)
	}
}
func (m *Module) heap_ptr_get() uint32 {
	addr, err := m.module.ExportedFunction("opa_heap_ptr_get").Call(m.ctx)
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
func (m *Module) value_add_path(base_value_addr, path_value_addr, value_addr uint32) {
	ret, err := m.module.ExportedFunction("opa_value_add_path").Call(m.ctx, uint64(base_value_addr), uint64(path_value_addr), uint64(value_addr))
	if err != nil {
		log.Panic(err)
	}
	if ret[0] == 1 {
		log.Panic("OPA_ERR_INTERNAL")
	} else if ret[0] == 2 {
		log.Println("OPA_ERR_INVALID_TYPE")
	} else if ret[0] == 3 {
		log.Println("OPA_ERR_INVALID_PATH")
	}
}
func (m *Module) value_remove_path(base_value_addr, path_value_addr uint32) {
	ret, err := m.module.ExportedFunction("opa_value_remove_path").Call(m.ctx, uint64(base_value_addr), uint64(path_value_addr))
	if err != nil {
		log.Panic(err)
	}
	if ret[0] == 1 {
		log.Panic("OPA_ERR_INTERNAL")
	} else if ret[0] == 2 {
		log.Println("OPA_ERR_INVALID_TYPE")
	} else if ret[0] == 3 {
		log.Println("OPA_ERR_INVALID_PATH")
	}
}
func (m *Module) opa_eval(entrypoint_id, data, input, input_len, heap_ptr, format uint32) uint32 {
	addr, err := m.module.ExportedFunction("opa_eval").Call(m.ctx, 0, uint64(entrypoint_id), uint64(data), uint64(input), uint64(input_len), uint64(heap_ptr), uint64(format))
	if err != nil {
		log.Panic(err)
	}
	return uint32(addr[0])
}
