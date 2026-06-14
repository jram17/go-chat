package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jram17/go-chat/internal/crypto"
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

	peerKeys := make(map[string][]byte) //username to public key

	//
	//
	// =======SETUP============
	//
	//
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
	var privateKey []byte
	//GENERATING PUBLIC PRIVATE KEYSSS
	privateKey, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		panic(err)
	}
	keys := protocol.Envelope{
		Type:      protocol.MessageTypeKeyExchange,
		From:      username,
		Payload:   pub,
		Timestamp: time.Now().Unix(),
	}
	env, err = protocol.Encode(keys)
	if err != nil {
		fmt.Println("Encoding error (client-side)", err)
	}
	_, err = conn.Write(env)
	if err != nil {
		fmt.Println("write error (client-side):", err)

	}

	//
	//
	//===========WRITE==============
	//
	//

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

			case protocol.MessageTypePrivate:
				senderPub, ok := peerKeys[env.From]
				if !ok {
					fmt.Println("sender public key not found")
					continue
				}
				key, err := crypto.ComputeSharedSecret(privateKey, senderPub)
				if err != nil {
					fmt.Println("key derivation failed:", err)
					continue
				}

				plaintext, err := crypto.Decrypt(key, env.Payload)
				if err != nil {
					fmt.Println("decryption failed:", err)
					continue
				}
				fmt.Printf(
					"[PRIVATE][%s]: %s\n",
					env.From,
					string(plaintext),
				)
				fmt.Printf(
					"SERVER PAYLOAD: %x\n",
					env.Payload,
				)

			case protocol.MessageTypeKeyExchange:
				peerKeys[env.From] = env.Payload
				fmt.Printf(
					"Stored public key for %s\n",
					env.From,
				)
			}

		}
	}()
	//
	// =======READ==========
	//
	//
	//
	for scanner.Scan() {
		var env protocol.Envelope
		input := scanner.Text()

		if strings.HasPrefix(input, "/msg") {
			parts := strings.SplitN(input, " ", 3)
			if len(parts) < 3 {
				fmt.Println("Usage: /msg <end-user> <message>")
				continue
			}
			recipient := parts[1]
			message := parts[2]
			//get the recipient public key
			recipientPub, ok := peerKeys[recipient]
			if !ok {
				fmt.Println("user public key not found!!")
				continue
			}
			key, err := crypto.ComputeSharedSecret(privateKey, recipientPub)
			if err != nil {
				fmt.Println("key derivation failed!!:", err)
				continue
			}
			ciphertext, err := crypto.Encrypt(key, []byte(message))
			if err != nil {
				fmt.Println("encryption failed:", err)
				continue
			}

			env = protocol.Envelope{
				Type:      protocol.MessageTypePrivate,
				From:      username,
				To:        recipient,
				Payload:   ciphertext,
				Timestamp: time.Now().Unix(),
			}
		} else {
			//regular chat
			env = protocol.Envelope{
				Type:      protocol.MessageTypeChat,
				From:      username,
				Payload:   []byte(input),
				Timestamp: time.Now().Unix(),
			}
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
