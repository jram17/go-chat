package main

import (
	"crypto/tls"
	"fmt"

	"github.com/jram17/go-chat/internal/server"
)

func main() {
	hub := server.NewHub()
	go hub.Run()

	//making everything tls now boiii

	cert, err := tls.LoadX509KeyPair(
		"certs/server.crt",
		"certs/server.key",
	)
	if err != nil {
		panic(err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{
			cert,
		},
	}
	listener, err := tls.Listen(
		"tcp",
		":9000",
		config,
	)
	if err!=nil{
		panic(err)
	}

	defer listener.Close()
	fmt.Println("server is running on:", 9000)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept err:", err)
			continue // for other connections
		}

		client := server.NewClient(conn, hub)
		//so basically client writes hub forwards !!! client reads !!!
		//hub.Register(client)
		go client.WritePump()
		go client.ReadPump()

	}
}
