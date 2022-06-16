package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tetratelabs/wazero"
)

type vmOpts struct {
	policy    []byte
	data      []byte
	memoryMin uint32
}
type VM struct {
	runtime                                     wazero.Runtime
	ctx                                         context.Context
	module                                      Module
	policy, data                                []byte
	parsedDataAddr, parsedInputAddr, outputAddr int32
	memoryMin                                   uint32
}

func newVM(opts vmOpts) VM {
	vm := VM{}
	vm.ctx = context.Background()
	vm.runtime = wazero.NewRuntime()
	vm.policy = opts.policy
	vm.data = opts.data
	vm.LoadPolicy()
	vm.LoadData()
	return vm
}
func (vm *VM) SetPolicy(policy []byte) {
	vm.policy = policy
}
func (vm *VM) LoadPolicy() {
	vm.module = newModule(moduleOpts{name: "opa", policy: vm.policy, ctx: vm.ctx, MinMemSize: 10, vm: vm}, vm.runtime)
}
func (vm *VM) SetData(data []byte) {
	vm.data = data
}
func (vm *VM) LoadData() {
	dLoc, err := vm.module.writeMem(vm.data)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(vm.module.readStr(dLoc))
	dat, err := vm.module.module.ExportedFunction("opa_json_parse").Call(vm.ctx, uint64(dLoc), uint64(len(vm.data)))
	if err != nil {
		log.Panic(err)
	}
	vm.parsedDataAddr = int32(dat[0])
}
func (vm *VM) Eval(input []byte) string {

	mod := vm.module.module
	dLoc, err := vm.module.writeMem(input)
	if err != nil {
		log.Panic(err)
	}
	dat, err := vm.module.module.ExportedFunction("opa_json_parse").Call(vm.ctx, uint64(dLoc), uint64(len(input)))
	if err != nil {
		log.Panic(err)
	}
	vm.parsedInputAddr = int32(dat[0])
	eCtx, err := mod.ExportedFunction("opa_eval_ctx_new").Call(vm.ctx)
	if err != nil {
		log.Panicln(err)
	}
	fmt.Println(vm.module.readStr(dLoc))
	_, err = mod.ExportedFunction("opa_eval_ctx_set_input").Call(vm.ctx, eCtx[0], uint64(vm.parsedInputAddr))
	if err != nil {
		log.Panicln(err)
	}
	vm.LoadData()
	_, err = mod.ExportedFunction("opa_eval_ctx_set_data").Call(vm.ctx, eCtx[0], uint64(vm.parsedDataAddr))
	if err != nil {
		log.Panicln(err)
	}
	_, err = mod.ExportedFunction("eval").Call(vm.ctx, eCtx[0])
	if err != nil {
		log.Panicln(err)
	}
	resLoc, err := mod.ExportedFunction("opa_eval_ctx_get_result").Call(vm.ctx, eCtx[0])
	if err != nil {
		log.Panicln(err)
	}
	strLoc, err := mod.ExportedFunction("opa_json_dump").Call(vm.ctx, resLoc[0])
	if err != nil {
		log.Panicln(err)
	}
	return vm.module.readStr(uint32(strLoc[0]))
}
func (vm *VM) getData() string {
	dataLoc, err := vm.module.module.ExportedFunction("opa_json_dump").Call(vm.ctx, uint64(vm.parsedDataAddr))
	if err != nil {
		log.Panicln(err)
	}
	return vm.module.readStr(uint32(dataLoc[0]))
}
func (vm *VM) getPolicy() string {

	out := ""
	for _, b := range vm.policy {
		out += string(b)
	}
	return out
}
