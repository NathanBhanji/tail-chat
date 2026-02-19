package protocol

import (
	"bytes"
	"testing"
)

func TestWrapUnwrap(t *testing.T) {
	hs := &Handshake{
		PublicKey: []byte("test-key-32-bytes-long-padding!!"),
		Hostname:  "alice-macbook",
		Version:   "1.0.0",
	}

	env, err := Wrap(TypeHandshake, hs)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	if env.Type != TypeHandshake {
		t.Fatalf("type: got %d, want %d", env.Type, TypeHandshake)
	}

	got, err := Unwrap[Handshake](env)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}

	if got.Hostname != hs.Hostname {
		t.Fatalf("hostname: got %q, want %q", got.Hostname, hs.Hostname)
	}
}

func TestReadWriteMessage(t *testing.T) {
	msg := &ChatMessage{
		ID:         "msg-123",
		Ciphertext: []byte("encrypted-data-here"),
		Timestamp:  1700000000,
	}

	env, _ := Wrap(TypeChat, msg)

	var buf bytes.Buffer
	if err := WriteMessage(&buf, env); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	got, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if got.Type != TypeChat {
		t.Fatalf("type: got %d, want %d", got.Type, TypeChat)
	}

	chatMsg, err := Unwrap[ChatMessage](got)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}

	if chatMsg.ID != msg.ID {
		t.Fatalf("id: got %q, want %q", chatMsg.ID, msg.ID)
	}
}

func TestMaxMessageSize(t *testing.T) {
	// Create an oversized envelope
	huge := &ChatMessage{
		ID:         "big",
		Ciphertext: make([]byte, MaxMessageSize+1),
	}

	env, _ := Wrap(TypeChat, huge)
	var buf bytes.Buffer

	err := WriteMessage(&buf, env)
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
}
