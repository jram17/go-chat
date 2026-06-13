package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jram17/go-chat/internal/protocol"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Println("connected to server ")
	fmt.Println("this is your conn port:", conn.LocalAddr())

	//start a backgroud go routine for reading from server
	//before listing to the keyboard
	go func() {
		reader := bufio.NewReader(conn)
		for {
			env, err := protocol.Decode(reader)
			if err != nil {
				fmt.Println("client dissconnected:", conn.RemoteAddr())
				return
			}
			switch env.Type {
				
			case protocol.MessageTypeJoin:
				fmt.Printf("%s joined the chat\n", env.From)

			case protocol.MessageTypeLeave:
				fmt.Printf("%s left the chat\n", env.From)

			case protocol.MessageTypeChat:
				fmt.Printf("[%s]: %s\n",
					env.From,
					string(env.Payload))
			}

		}
	}()

	//to read from the terminal
	fmt.Println("ENTER YOUR NAME:")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := scanner.Text()
	join := protocol.Envelope{
		Type:      protocol.MessageTypeJoin,
		From:      username,
		Timestamp: time.Now().Unix(),
	}

	env, err := protocol.Encode(join)
	if err != nil {
		fmt.Println("Encoding err:", err)
		return
	}
	_, err = conn.Write(env)
	if err != nil {
		fmt.Println("write error (client-side):", err)
	}

	for scanner.Scan() {
		env := protocol.Envelope{
			Type:      protocol.MessageTypeChat,
			From:      username,
			Payload:   []byte(scanner.Text()),
			Timestamp: time.Now().Unix(),
		}
		encoded, err := protocol.Encode(env)
		if err != nil {
			fmt.Println("Encoding error (client side):", err)
			return
		}
		_, err = conn.Write(encoded)
		if err != nil {
			fmt.Println("write error (client side):", err)
			return
		}
	}

}
