// for set.go we will be implementing a basic code for hashmap where we'll be storing key value pair

package commandhandler

import (
	resp "Distributed_Cache/Resp"
	"sync"
)

var SETs = map[string]string{}
var SETsMu = sync.RWMutex{}
var HSETs = map[string]map[string]string{}
var HSETsMu = sync.RWMutex{}

func set(args []resp.Value) resp.Value {
	if len(args) != 2 {
		return resp.Value{Typ: "error", Str: "ERR wrong number of arguments for 'set' command"}
	}
	key := args[0].Bulk
	value := args[1].Bulk

	SETsMu.Lock()
	SETs[key] = value
	SETsMu.Unlock()

	return resp.Value{Typ: "string", Str: "OK"}
}



func hset(args []resp.Value) resp.Value {
	if len(args) != 3 {
		return resp.Value{Typ: "error", Str : "ERR wrong number of arguments for 'hset' command"}
	}
	hash:= args[0].Bulk
	key:= args[1].Bulk
	value:= args[2].Bulk


	HSETsMu.Lock()
	if _, ok:= HSETs[hash]; !ok {
		HSETs[hash] = map[string]string{}
	}
	HSETs[hash][key] = value
	HSETsMu.Unlock()

	return resp.Value{Typ: "string", Str: "OK"}
}



