package commandhandler

import (
	resp "Distributed_Cache/resp"
)

var Handlers = map[string]func([]resp.Value) resp.Value{
	"PING": ping,
	"SET":  set,
	"GET":  get,
	"HSET": hset,
}

func ping(args []resp.Value) resp.Value {
	if len(args) == 0 {
		return resp.Value{Typ: "string", Str: "Pong"}
	}
	return resp.Value{Typ: "bulk", Bulk: args[0].Bulk}
}
