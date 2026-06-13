package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	MessageTypeChat = iota + 1
	MessageTypeJoin
	MessageTypeLeave
	MessageTypeKeyExchange
	MessageTypePrivate
	MessageTypeUserList
)

type Envelope struct {
	Type      int    `json:"type"`
	From      string `json:"from"`
	To        string `json:"to,omitempty"`
	Payload   []byte `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

func Encode(env Envelope) ([]byte, error) {
	data := Envelope{
		Type:      env.Type,
		From:      env.From,
		To:        env.To,
		Payload:   env.Payload,
		Timestamp: env.Timestamp,
	}
	jsonPayload,err := json.Marshal(data)
	if err!=nil{
		return nil, fmt.Errorf("failed to marshall json: %w", err)
	}
	length:=len(jsonPayload)
	header := make([]byte,4)
	//put the lenght in the header in binary
	binary.BigEndian.PutUint32(header,uint32(length))

	envelope:=append(header,jsonPayload...)
	return envelope,nil
}

func Decode(reader io.Reader)(Envelope,error){
	var env Envelope
	header:= make([]byte,4)
	_,err:= io.ReadFull(reader,header)
	if err!=nil{
		return env, fmt.Errorf("failed to read length header: %w", err)
	}
	length := binary.BigEndian.Uint32(header)
	jsonPayload :=make([]byte,length)
	_,err = io.ReadFull(reader,jsonPayload)
	if err!=nil{
		return env, fmt.Errorf("failed to read json-payload : %w", err)
	}
	//unmarshall the json payload
	err = json.Unmarshal(jsonPayload,&env)
	if err!=nil{
		return env,  fmt.Errorf("failed to read unmarshall json-payload: %w", err)
	}
	return env,nil
}