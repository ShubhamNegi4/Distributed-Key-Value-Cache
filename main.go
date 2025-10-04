package main

import (
	persist "Distributed_Cache/Aof"
	resp "Distributed_Cache/Resp"
	handle "Distributed_Cache/commandHandler"
	"fmt"
	"net"
	"strings"
)

func main() {
	fmt.Println("Listening on port: 6379")
	//Create a new server
	l, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	aof, err := persist.NewAof("database.aof")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer aof.Close()

	// Replay AOF file on startup
	err = aof.Read(func(value resp.Value) {
		command := strings.ToUpper(value.Array[0].Bulk)
		args := value.Array[1:]

		handler, ok := handle.Handlers[command]
		if !ok {
			fmt.Println("Invalid command: ", command)
			return
		}

		handler(args)
	})
	if err != nil {
		fmt.Printf("Error replaying AOF file: %v\n", err)
	}

	// Accept connections in a loop
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Handle each connection in a goroutine
		go handleConnection(conn, aof)
	}
}

func handleConnection(conn net.Conn, aof *persist.Aof) {
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
		if command == "SET" || command == "HSET" {
			aof.Write(value)
		}
		result := handler(args)
		conn.Write(result.Marshal())
	}
}
