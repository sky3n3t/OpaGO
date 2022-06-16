package main

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
func (m *Module) opaAbort(ptr int32) {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b, err := m.readMemByte(uint32(ptr) + index)
		if err != nil {
			log.Panic(err)
		}
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
			fmt.Println("test1")
			log.Panic(err)
		}
		data := m.readStr(uint32(serialized[0]))
		pTer, err := ast.ParseTerm(string(data))
		if err != nil {
			fmt.Println("test1")
			log.Panic(err)
		}
		pArgs = append(pArgs, pTer)
	}
	err := m.builtinT[args[0]](*m.tCTX, pArgs, func(t *ast.Term) error {
		output = t
		return nil
	})
	if err != nil {
		fmt.Println("test2")
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
		fmt.Println("test1")
		return 0
	}
	outB := []byte(output.String())
	loc, err := m.writeMem(outB)
	if err != nil {
		fmt.Println("test1")
		log.Panic(err)
	}
	addr, err := m.module.ExportedFunction("opa_value_parse").Call(m.ctx, uint64(loc), uint64(len(outB)))
	if err != nil {
		fmt.Println("test1")
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
		b, err := m.readMemByte(uint32(ptr) + index)
		if err != nil {
			log.Panic(err)
		}
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
func (m *Module) readMem(offset, length uint32) ([]byte, error) {
	data, overflow := m.env.Memory().Read(m.ctx, offset, length)
	if !overflow {
		return []byte{}, errors.New(fmt.Sprintf("memory index out of range"))
	}
	return data, nil
}
func (m *Module) readMemByte(offset uint32) (byte, error) {
	data, overflow := m.env.Memory().ReadByte(m.ctx, offset)
	if !overflow {
		return 0b0, errors.New(fmt.Sprintf("memory index out of range"))
	}
	return data, nil
}
func (m *Module) writeMem(data []byte) (uint32, error) {

	addr, err := m.module.ExportedFunction("opa_malloc").Call(m.ctx, uint64(len(data)))
	if err != nil {
		fmt.Println("test1")
		return 0, err
	}
	overflow := m.env.Memory().Write(m.ctx, uint32(addr[0]), data)
	if !overflow {
		return 0, errors.New(fmt.Sprintf("memory index out of range"))
	}
	return uint32(addr[0]), nil
}
func (m *Module) readStr(loc uint32) string {
	bytes := []byte{}
	var index uint32 = 0
	for ok := true; ok; {
		b, _ := m.readMemByte(loc + index)
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
