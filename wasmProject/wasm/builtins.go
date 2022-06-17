package wasm

import (
	"fmt"
	"log"
	"strconv"

	"github.com/open-policy-agent/opa/topdown"
)

func newBuiltinTable(mod Module) map[int32]topdown.BuiltinFunc {
	builtinStrAddr, err := mod.module.ExportedFunction("builtins").Call(mod.ctx)
	if err != nil {
		log.Panicln(err)
	}
	builtinsJSON, err := mod.module.ExportedFunction("opa_json_dump").Call(mod.ctx, builtinStrAddr[0])
	builtinStr := mod.readStr(uint32(builtinsJSON[0]))
	builtinNameMap := parseJsonBuiltinString(builtinStr)
	builtinIdMap, err := getFuncs(builtinNameMap)
	if err != nil {
		log.Panic(err)
	}
	return builtinIdMap
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
