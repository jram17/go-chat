package protocol

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := Envelope{
		Type:      MessageTypeChat,
		From:      "alice",
		To:        "bob",
		Payload:   []byte("hello world"),
		Timestamp: 1234567890,
	}

	encoded, err := Encode(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	reader := bytes.NewReader(encoded)
	decoded, err := Decode(reader)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %d, want %d", decoded.Type, original.Type)
	}
	if decoded.From != original.From {
		t.Errorf("From mismatch: got %q, want %q", decoded.From, original.From)
	}
	if decoded.To != original.To {
		t.Errorf("To mismatch: got %q, want %q", decoded.To, original.To)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("Payload mismatch: got %q, want %q", decoded.Payload, original.Payload)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", decoded.Timestamp, original.Timestamp)
	}
}

func TestEncodeDecodeAllTypes(t *testing.T) {
	types := []int{
		MessageTypeChat,
		MessageTypeJoin,
		MessageTypeLeave,
		MessageTypeKeyExchange,
		MessageTypePrivate,
		MessageTypeUserList,
	}

	for _, mt := range types {
		env := Envelope{
			Type:      mt,
			From:      "testuser",
			Payload:   []byte("test payload"),
			Timestamp: 9999,
		}
		encoded, err := Encode(env)
		if err != nil {
			t.Fatalf("Encode failed for type %d: %v", mt, err)
		}
		decoded, err := Decode(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("Decode failed for type %d: %v", mt, err)
		}
		if decoded.Type != mt {
			t.Errorf("type %d: got %d", mt, decoded.Type)
		}
	}
}

func TestDecodeEmptyReader(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	_, err := Decode(reader)
	if err == nil {
		t.Fatal("expected error on empty reader")
	}
}

func TestDecodeTruncatedPayload(t *testing.T) {
	env := Envelope{
		Type:    MessageTypeChat,
		From:    "alice",
		Payload: []byte("hello"),
	}
	encoded, _ := Encode(env)

	// Truncate: only include header + partial payload
	truncated := encoded[:6]
	_, err := Decode(bytes.NewReader(truncated))
	if err == nil {
		t.Fatal("expected error on truncated payload")
	}
}

func TestEncodeDecodeEmptyPayload(t *testing.T) {
	env := Envelope{
		Type:      MessageTypeJoin,
		From:      "bob",
		Timestamp: 111,
	}
	encoded, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if decoded.From != "bob" {
		t.Errorf("From mismatch: got %q", decoded.From)
	}
}
