package wasm

type Pool struct {
	VMs         map[string]VM
	entrypoints map[string]struct {
		vm            string
		entrypoint_id int32
	}
}
