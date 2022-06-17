package wasm

type Pool struct {
	vms         map[string]VM
	entrypoints map[string]struct {
		vm            string
		entrypoint_id int32
	}
}
