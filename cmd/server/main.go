package main

import (
	"fmt"
	"net"

	"github.com/jram17/go-chat/internal/server"
)

func main() {
	hub:= server.NewHub()
	go hub.Run()
	listener, err := net.Listen("tcp",":9000")
	if err!=nil{
		panic(err)
	}
	defer listener.Close()
	fmt.Println("server is running on:",9000)
	for{
		conn,err:=listener.Accept()
		if err!=nil{
			fmt.Println("Accept err:",err)
			continue // for other connections
		}
		

		client:= server.NewClient(conn,hub)
		//so basically client writes hub forwards !!! client reads !!!
		//hub.Register(client)
		go client.WritePump()
		go client.ReadPump()
		
	}
}