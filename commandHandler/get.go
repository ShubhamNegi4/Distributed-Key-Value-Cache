package commandhandler



import (
	resp "Distributed_Cache/Resp"

)



func get(args []resp.Value) resp.Value {
	if len(args) != 1 {
		return resp.Value{Typ: "error", Str: "ERR wrong number of arguments for 'get' command"}
	}
	key:= args[0].Bulk
	SETsMu.RLock()
	value, ok := SETs[key]
	SETsMu.RUnlock()

	if !ok {
		return resp.Value{Typ: 	"null"}
	}
	return resp.Value{Typ: "bulk", Bulk: value}
}



func hget(args []resp.Value) resp.Value {
	if len(args) != 2 {
		return resp.Value{Typ:"error", Str: "ERR wrong number of arguments for 'hget' command"}
	}
	hash:= args[0].Bulk
	key:= args[1].Bulk

	HSETsMu.Lock()
	value, ok := HSETs[hash][key]

	if !ok {
		return resp.Value{Typ:"null"}
	}

	return resp.Value{Typ:"bulk", Bulk: value}
}