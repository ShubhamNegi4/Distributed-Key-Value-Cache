package main

import (
	resp "Distributed_Cache/Resp"
	handle "Distributed_Cache/commandHandler"
	"fmt"
	"net"
	"strings"
)

func main() {
	fmt.Println("Listening on port: 6379")
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := l.Accept()
	if err != nil {
		fmt.Println(err)
		return
	}

	defer conn.Close()

	for {
		r := resp.NewResp(conn)
		value, err := r.Read()
		if err != nil {
			fmt.Println(err)
			return
		}
		if value.Typ != "array" {
			fmt.Println("Invalid request, expected array")
			continue
		}
		if len(value.Array) == 0 {
			fmt.Println("Invalid request, expected array length > 0")
			continue
		}
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]

		handler, ok := handle.Handlers[command]
		if !ok {
			// write RESP error back without noisy console logs
			conn.Write(resp.Value{Typ: "error", Str: "ERR unknown command"}.Marshal())
			continue
		}
		result := handler(args)
		conn.Write(result.Marshal())
	}
}
