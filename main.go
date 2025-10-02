package main

import (
	resp "Distributed_Cache/Resp"
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
		_ = command
		_ = value.Array[1:]

		// Command handlers are not yet implemented. Reply with RESP error for now.
		errResp := resp.Value{Typ: "error", Str: "ERR unknown command"}
		_, _ = conn.Write(errResp.Marshal())
	}
}
