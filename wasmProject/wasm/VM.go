package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

type VmOpts struct {
	Policy    []byte
	Data      []byte
	MemoryMin int
	Pool      *Pool
}
type VM struct {
	runtime                         wazero.Runtime
	ctx                             context.Context
	module                          Module
	policy, data                    []byte
	parsedDataAddr, parsedInputAddr uint32
	entrypoints                     map[string]int32
	memoryMin                       int
	pool                            *Pool
}
func NewVM(opts VmOpts) VM {
	vm := VM{}
	vm.ctx = context.Background()
	vm.runtime = wazero.NewRuntime()
	vm.policy = opts.Policy
	vm.data = opts.Data
	vm.memoryMin = opts.MemoryMin
	vm.LoadPolicy()
	vm.LoadData()
	fmt.Println(vm.GetEntrypoints())
	return vm
}
func (vm *VM) SetPolicy(policy []byte) {
	vm.policy = policy
}
func (vm *VM) Name() string {
	return vm.module.name
}
func (vm *VM) GetEntrypoints() map[string]int32 {
	return vm.module.GetEntrypoints()
}
func (vm *VM) Module() Module {
	return vm.module
}
func (vm *VM) LoadPolicy() {
	vm.module = newModule(moduleOpts{name: "opa", policy: vm.policy, ctx: vm.ctx, MinMemSize: vm.memoryMin, vm: vm}, vm.runtime)
}
func (vm *VM) SetData(data []byte) {
	vm.data = data
}
func (vm *VM) LoadData() {
	dLoc := vm.module.writeMem(vm.data)

	dat := vm.module.json_parse(dLoc, uint32(len(vm.data)))
	vm.parsedDataAddr = dat
}
func (vm *VM) Eval(input []byte) string {
	dLoc := vm.module.writeMem(input)
	dat := vm.module.json_parse(dLoc, uint32(len(input)))
	vm.parsedInputAddr = dat
	eCtx := vm.module.eval_ctx_new()
	vm.module.eval_ctx_set_input(eCtx, vm.parsedInputAddr)
	vm.LoadData()
	vm.module.eval_ctx_set_data(eCtx, vm.parsedDataAddr)
	vm.module.eval(eCtx)
	resLoc := vm.module.eval_ctx_get_result(eCtx)
	return vm.module.fromRegoJSON(resLoc)
}
func (vm *VM) GetData() string {
	dataLoc := vm.module.json_dump(vm.parsedDataAddr)
	return vm.module.readStr(dataLoc)
}
func (vm *VM) GetPolicy() string {
	out := ""
	for _, b := range vm.policy {
		out += string(b)
	}
	return out
}
