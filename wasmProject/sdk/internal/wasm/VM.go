package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
	"github.com/tetratelabs/wazero"
)

type vmOpts struct {
	policy         []byte
	data           []byte
	parsedData     []byte
	parsedDataAddr int32
	memoryMin      uint32
	memoryMax      uint32
}
type VM struct {
	runtime              *wazero.Runtime
	ctx                  context.Context
	module               Module
	policy               []byte
	parsedInputAddr      uint32
	memoryMin            int
	memoryMax            int
	abiMajorVersion      int32
	abiMinorVersion      int32
	entrypointIDs        map[string]int32
	baseHeapPtr          int32
	dataAddr             int32
	evalHeapPtr          int32
	evalOneOff           func(context.Context, int32, int32, int32, int32, int32) (int32, error)
	eval                 func(context.Context, int32) error
	evalCtxGetResult     func(context.Context, int32) (int32, error)
	evalCtxNew           func(context.Context) (int32, error)
	evalCtxSetData       func(context.Context, int32, int32) error
	evalCtxSetInput      func(context.Context, int32, int32) error
	evalCtxSetEntrypoint func(context.Context, int32, int32) error
	heapPtrGet           func(context.Context) (int32, error)
	heapPtrSet           func(context.Context, int32) error
	jsonDump             func(context.Context, int32) (int32, error)
	jsonParse            func(context.Context, int32, int32) (int32, error)
	valueDump            func(context.Context, int32) (int32, error)
	valueParse           func(context.Context, int32, int32) (int32, error)
	malloc               func(context.Context, int32) (int32, error)
	free                 func(context.Context, int32) error
	valueAddPath         func(context.Context, int32, int32, int32) (int32, error)
	valueRemovePath      func(context.Context, int32, int32) (int32, error)
}

func newVM(opts vmOpts, runtime *wazero.Runtime) (*VM, error) {
	vm := VM{}
	vm.ctx = context.Background()
	vm.runtime = runtime
	vm.policy = opts.policy
	vm.memoryMin = int(opts.memoryMin)
	vm.memoryMax = int(opts.memoryMax)
	modOpts := moduleOpts{policy: opts.policy, ctx: vm.ctx, minMemSize: int(opts.memoryMin), maxMemSize: int(opts.memoryMax), vm: &vm}
	vm.module = newModule(modOpts, *runtime)
	vm.abiMajorVersion = vm.module.wasm_abi_version()
	vm.abiMinorVersion = vm.module.wasm_abi_minor_version()
	vm.entrypointIDs = vm.GetEntrypoints()
	vm.evalOneOff = vm.module.opa_eval
	vm.eval = vm.module.eval
	vm.evalCtxGetResult = vm.module.eval_ctx_get_result
	vm.evalCtxNew = vm.module.eval_ctx_new
	vm.evalCtxSetData = vm.module.eval_ctx_set_data
	vm.evalCtxSetInput = vm.module.eval_ctx_set_input
	vm.evalCtxSetEntrypoint = vm.module.eval_ctx_set_entrypoint
	vm.heapPtrGet = vm.module.heap_ptr_get
	vm.heapPtrSet = vm.module.heap_ptr_set
	vm.jsonDump = vm.module.json_dump
	vm.jsonParse = vm.module.json_parse
	vm.valueDump = vm.module.value_dump
	vm.valueParse = vm.module.value_parse
	vm.malloc = vm.module.malloc
	vm.free = vm.module.free
	vm.valueAddPath = vm.module.value_add_path
	vm.valueRemovePath = vm.module.value_remove_path
	var err error
	if _, err = vm.malloc(vm.ctx, 0); err != nil {
		return nil, err
	}
	if vm.baseHeapPtr, err = vm.getHeapState(vm.ctx); err != nil {
		return nil, err
	}
	if opts.parsedData != nil {
		vm.dataAddr = int32(vm.module.writeMem(opts.parsedData))
		vm.baseHeapPtr = vm.dataAddr
		vm.evalHeapPtr = vm.baseHeapPtr + int32(len(opts.parsedData))
		err := vm.setHeapState(vm.ctx, vm.evalHeapPtr)
		if err != nil {
			return nil, err
		}
	} else if opts.data != nil {
		if vm.dataAddr, err = vm.toRegoJSON(vm.ctx, opts.data, true); err != nil {
			return nil, err
		}
	}
	return &vm, nil
}
func (i *VM) SetPolicyData(ctx context.Context, opts vmOpts) error {

	if !bytes.Equal(opts.policy, i.policy) {
		// Swap the instance to a new one, with new policy.
		i.module.module.Close(i.ctx)
		i.module.env.Close(i.ctx)
		n, err := newVM(opts, i.runtime)
		if err != nil {
			return err
		}

		*i = *n
		return nil
	}

	i.dataAddr = 0

	var err error
	if err = i.setHeapState(ctx, i.baseHeapPtr); err != nil {
		return err
	}

	if opts.parsedData != nil {
		i.dataAddr = int32(i.module.writeMem(opts.parsedData))
	} else if opts.data != nil {
		if i.dataAddr, err = i.toRegoJSON(ctx, opts.data, true); err != nil {
			return err
		}
	}

	if i.evalHeapPtr, err = i.getHeapState(ctx); err != nil {
		return err
	}

	return nil
}

type abortError struct {
	message string
}

type cancelledError struct {
	message string
}

// Println is invoked if the policy WASM code calls opa_println().
func (i *VM) Println(arg int32) {
	data := i.module.readFrom(arg)
	n := bytes.IndexByte(data, 0)
	if n == -1 {
		panic("invalid opa_println argument")
	}

	fmt.Printf("opa_println(): %s\n", string(data[:n]))
}

type builtinError struct {
	err error
}

// Entrypoints returns a mapping of entrypoint name to ID for use by Eval().
func (i *VM) Entrypoints() map[string]int32 {
	return i.entrypointIDs
}

// SetDataPath will update the current data on the VM by setting the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) SetDataPath(ctx context.Context, path []string, value interface{}) error {
	// Reset the heap ptr before patching the vm to try and keep any
	// new allocations safe from subsequent heap resets on eval.
	err := i.setHeapState(ctx, i.evalHeapPtr)
	if err != nil {
		return err
	}

	valueAddr, err := i.toRegoJSON(ctx, value, true)
	if err != nil {
		return err
	}

	pathAddr, err := i.toRegoJSON(ctx, path, true)
	if err != nil {
		return err
	}

	result, err := i.valueAddPath(ctx, i.dataAddr, pathAddr, valueAddr)
	if err != nil {
		return err
	}

	// We don't need to free the value, assume it is "owned" as part of the
	// overall data object now.
	// We do need to free the path

	if err := i.free(ctx, pathAddr); err != nil {
		return err
	}

	// Update the eval heap pointer to accommodate for any new allocations done
	// while patching.
	i.evalHeapPtr, err = i.getHeapState(ctx)
	if err != nil {
		return err
	}

	errc := result
	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

// RemoveDataPath will update the current data on the VM by removing the value at the
// specified path. If an error occurs the instance is still in a valid state, however
// the data will not have been modified.
func (i *VM) RemoveDataPath(ctx context.Context, path []string) error {
	pathAddr, err := i.toRegoJSON(ctx, path, true)
	if err != nil {
		return err
	}

	errc, err := i.valueRemovePath(ctx, i.dataAddr, pathAddr)
	if err != nil {
		return err
	}

	if err := i.free(ctx, pathAddr); err != nil {
		return err
	}

	if errc != 0 {
		return fmt.Errorf("unable to set data value for path %v, err=%d", path, errc)
	}

	return nil
}

// fromRegoJSON parses serialized JSON from the Wasm memory buffer into
// native go types.
func (i *VM) toRegoJSON(ctx context.Context, v interface{}, free bool) (int32, error) {
	var raw []byte
	switch v := v.(type) {
	case []byte:
		raw = v
	case *ast.Term:
		raw = []byte(v.String())
	case ast.Value:
		raw = []byte(v.String())
	default:
		var err error
		raw, err = json.Marshal(v)
		if err != nil {
			return 0, err
		}
	}

	n := int32(len(raw))
	p := int32(i.module.writeMem(raw))

	addr, err := i.valueParse(ctx, p, n)
	if err != nil {
		return 0, err
	}

	if free {
		if err := i.free(ctx, p); err != nil {
			return 0, err
		}
	}

	return addr, nil
}
func (i *VM) getHeapState(ctx context.Context) (int32, error) {
	return i.heapPtrGet(ctx)
}

func (i *VM) setHeapState(ctx context.Context, ptr int32) error {
	return i.heapPtrSet(ctx, ptr)
}
func (vm *VM) cloneDataSegment() (int32, []byte) {
	srcData := vm.module.readMem(uint32(vm.baseHeapPtr), uint32(vm.evalHeapPtr-vm.baseHeapPtr))
	patchedData := make([]byte, len(srcData))
	copy(patchedData, srcData)
	return vm.dataAddr, patchedData
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
func (i *VM) Eval(ctx context.Context,
	entrypoint int32,
	input *interface{},
	metrics metrics.Metrics,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) ([]byte, error) {
	if i.abiMinorVersion < int32(2) {
		return i.evalCompat(ctx, entrypoint, input, metrics, seed, ns, iqbCache, ph, capabilities)
	}

	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	inputAddr, inputLen := int32(0), int32(0)

	// NOTE: we'll never free the memory used for the input string during
	// the one evaluation, but we'll overwrite it on the next evaluation.
	heapPtr := i.evalHeapPtr

	if input != nil {
		metrics.Timer("wasm_vm_eval_prepare_input").Start()
		var raw []byte
		switch v := (*input).(type) {
		case []byte:
			raw = v
		case *ast.Term:
			raw = []byte(v.String())
		case ast.Value:
			raw = []byte(v.String())
		default:
			var err error
			raw, err = json.Marshal(v)
			if err != nil {
				return nil, err
			}
		}
		inputAddr = int32(i.module.writeMem(raw))
		metrics.Timer("wasm_vm_eval_prepare_input").Stop()
	}

	// Setting the ctx here ensures that it'll be available to builtins that
	// make use of it (e.g. `http.send`); and it will spawn a go routine
	// cancelling the builtins that use topdown.Cancel, when the context is
	// cancelled.
	i.module.Reset(ctx, seed, ns, iqbCache, ph, capabilities)

	metrics.Timer("wasm_vm_eval_call").Start()
	resultAddr, err := i.evalOneOff(ctx, int32(entrypoint), i.dataAddr, inputAddr, inputLen, heapPtr)
	if err != nil {
		return nil, err
	}
	metrics.Timer("wasm_vm_eval_call").Stop()

	data := i.module.readUntil(resultAddr, 0b0)

	return data, nil
}
func (i *VM) evalCompat(ctx context.Context,
	entrypoint int32,
	input *interface{},
	metrics metrics.Metrics,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) ([]byte, error) {
	metrics.Timer("wasm_vm_eval").Start()
	defer metrics.Timer("wasm_vm_eval").Stop()

	metrics.Timer("wasm_vm_eval_prepare_input").Start()

	// Setting the ctx here ensures that it'll be available to builtins that
	// make use of it (e.g. `http.send`); and it will spawn a go routine
	// cancelling the builtins that use topdown.Cancel, when the context is
	// cancelled.
	i.module.Reset(ctx, seed, ns, iqbCache, ph, capabilities)

	err := i.setHeapState(ctx, i.evalHeapPtr)
	if err != nil {
		return nil, err
	}

	// Parse the input JSON and activate it with the data.
	ctxAddr, err := i.evalCtxNew(ctx)
	if err != nil {
		return nil, err
	}

	if i.dataAddr != 0 {
		if err := i.evalCtxSetData(ctx, ctxAddr, i.dataAddr); err != nil {
			return nil, err
		}
	}

	if err := i.evalCtxSetEntrypoint(ctx, ctxAddr, int32(entrypoint)); err != nil {
		return nil, err
	}

	if input != nil {
		inputAddr, err := i.toRegoJSON(ctx, *input, false)
		if err != nil {
			return nil, err
		}

		if err := i.evalCtxSetInput(ctx, ctxAddr, inputAddr); err != nil {
			return nil, err
		}
	}
	metrics.Timer("wasm_vm_eval_prepare_input").Stop()

	// Evaluate the policy.
	metrics.Timer("wasm_vm_eval_execute").Start()
	err = i.eval(ctx, ctxAddr)
	metrics.Timer("wasm_vm_eval_execute").Stop()
	if err != nil {
		return nil, err
	}

	metrics.Timer("wasm_vm_eval_prepare_result").Start()
	resultAddr, err := i.evalCtxGetResult(ctx, ctxAddr)
	if err != nil {
		return nil, err
	}

	serialized, err := i.valueDump(ctx, resultAddr)
	if err != nil {
		return nil, err
	}

	data := i.module.readUntil(serialized, 0b0)

	metrics.Timer("wasm_vm_eval_prepare_result").Stop()

	// Skip free'ing input and result JSON as the heap will be reset next round anyway.

	return data, nil
}
func (vm *VM) GetData() string {
	dataLoc, _ := vm.module.json_dump(vm.ctx, int32(vm.dataAddr))
	return vm.module.readStr(uint32(dataLoc))
}
func (vm *VM) GetPolicy() string {
	out := ""
	for _, b := range vm.policy {
		out += string(b)
	}
	return out
}
