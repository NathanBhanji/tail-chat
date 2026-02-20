package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
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

func TestUnwrapBadPayload(t *testing.T) {
	env := &Envelope{
		Type:    TypeChat,
		Payload: json.RawMessage(`{invalid json`),
	}
	_, err := Unwrap[ChatMessage](env)
	if err == nil {
		t.Fatal("expected error for bad payload")
	}
}

func TestReadMessageTruncatedLength(t *testing.T) {
	// Only 2 bytes instead of 4 for length prefix
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x01})
	_, err := ReadMessage(&buf)
	if err == nil {
		t.Fatal("expected error for truncated length")
	}
}

func TestReadMessageTruncatedPayload(t *testing.T) {
	// Write a valid length prefix but truncated payload
	var buf bytes.Buffer
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, 100)
	buf.Write(lenBuf)
	buf.Write([]byte("short")) // only 5 bytes instead of 100
	_, err := ReadMessage(&buf)
	if err == nil {
		t.Fatal("expected error for truncated payload")
	}
}

func TestReadMessageOversizedLength(t *testing.T) {
	var buf bytes.Buffer
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, MaxMessageSize+1)
	buf.Write(lenBuf)
	_, err := ReadMessage(&buf)
	if err == nil {
		t.Fatal("expected error for oversized length")
	}
}

func TestReadMessageBadJSON(t *testing.T) {
	// Valid length prefix but invalid JSON
	payload := []byte("not json at all")
	var buf bytes.Buffer
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
	buf.Write(lenBuf)
	buf.Write(payload)
	_, err := ReadMessage(&buf)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestWrapAllMessageTypes(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		msg     any
	}{
		{"Handshake", TypeHandshake, &Handshake{PublicKey: []byte("key"), Hostname: "test"}},
		{"Chat", TypeChat, &ChatMessage{ID: "1", Ciphertext: []byte("ct")}},
		{"Ack", TypeAck, &Ack{MessageID: "1"}},
		{"Ping", TypePing, &Ping{Timestamp: 123}},
		{"Pong", TypePong, &Pong{Timestamp: 123}},
		{"GroupInvite", TypeGroupInvite, &GroupInvite{GroupID: "g1", GroupName: "test", Members: []string{"a"}}},
		{"GroupAccept", TypeGroupAccept, &GroupAccept{GroupID: "g1"}},
		{"GroupChat", TypeGroupChat, &GroupChat{ID: "1", GroupID: "g1", Sender: "a", Ciphertext: []byte("ct")}},
		{"GroupMembers", TypeGroupMembers, &GroupMembers{GroupID: "g1", Members: []string{"a", "b"}}},
		{"Typing", TypeTyping, &Typing{ChatKey: "bob", IsTyping: true}},
		{"Reaction", TypeReaction, &Reaction{MessageID: "1", ChatKey: "bob", Emoji: "👍", Sender: "alice"}},
		{"Status", TypeStatus, &Status{State: "busy"}},
		{"ReadReceipt", TypeReadReceipt, &ReadReceipt{MessageID: "1", ChatKey: "bob"}},
		{"FileOffer", TypeFileOffer, &FileOffer{ID: "t1", Filename: "test.txt", Size: 1024, Checksum: "abc123", ChatKey: "bob"}},
		{"FileData", TypeFileData, &FileData{ID: "t1", Offset: 0, Data: "aGVsbG8="}},
		{"FileComplete", TypeFileComplete, &FileComplete{ID: "t1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := Wrap(tt.msgType, tt.msg)
			if err != nil {
				t.Fatalf("Wrap: %v", err)
			}
			if env.Type != tt.msgType {
				t.Fatalf("type: got %d, want %d", env.Type, tt.msgType)
			}

			// Round-trip through write/read
			var buf bytes.Buffer
			if err := WriteMessage(&buf, env); err != nil {
				t.Fatalf("WriteMessage: %v", err)
			}
			got, err := ReadMessage(&buf)
			if err != nil {
				t.Fatalf("ReadMessage: %v", err)
			}
			if got.Type != tt.msgType {
				t.Fatalf("round-trip type: got %d, want %d", got.Type, tt.msgType)
			}
		})
	}
}

func TestMultipleMessagesInStream(t *testing.T) {
	var buf bytes.Buffer

	// Write 3 messages
	for i := 0; i < 3; i++ {
		msg := &ChatMessage{ID: string(rune('a' + i)), Ciphertext: []byte("data")}
		env, _ := Wrap(TypeChat, msg)
		if err := WriteMessage(&buf, env); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	// Read all 3 back
	for i := 0; i < 3; i++ {
		env, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		msg, _ := Unwrap[ChatMessage](env)
		expected := string(rune('a' + i))
		if msg.ID != expected {
			t.Fatalf("msg %d: got ID %q, want %q", i, msg.ID, expected)
		}
	}
}
