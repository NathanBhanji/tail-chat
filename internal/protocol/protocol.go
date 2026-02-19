package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// MessageType identifies the kind of message in an envelope.
type MessageType uint8

const (
	TypeHandshake    MessageType = 1
	TypeChat         MessageType = 2
	TypeAck          MessageType = 3
	TypePing         MessageType = 4
	TypePong         MessageType = 5
	TypeGroupInvite  MessageType = 6
	TypeGroupChat    MessageType = 7
	TypeGroupAccept  MessageType = 8
	TypeGroupMembers MessageType = 9
)

// Envelope wraps all messages with a type tag.
type Envelope struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Handshake is the first message exchanged to share public keys.
type Handshake struct {
	PublicKey []byte `json:"public_key"`
	Hostname  string `json:"hostname"`
	Version   string `json:"version"`
}

// ChatMessage is an encrypted 1:1 message.
type ChatMessage struct {
	ID         string `json:"id"`
	Ciphertext []byte `json:"ciphertext"`
	Timestamp  int64  `json:"timestamp"`
}

// Ack confirms receipt of a message.
type Ack struct {
	MessageID string `json:"message_id"`
}

// Ping/Pong for keepalive.
type Ping struct {
	Timestamp int64 `json:"timestamp"`
}

type Pong struct {
	Timestamp int64 `json:"timestamp"`
}

// GroupInvite is sent to invite a peer to a group chat.
type GroupInvite struct {
	GroupID   string   `json:"group_id"`
	GroupName string   `json:"group_name"`
	Members   []string `json:"members"` // hostnames
}

// GroupAccept is sent to accept a group invite.
type GroupAccept struct {
	GroupID string `json:"group_id"`
}

// GroupChat is an encrypted group message.
type GroupChat struct {
	ID         string `json:"id"`
	GroupID    string `json:"group_id"`
	Sender     string `json:"sender"`
	Ciphertext []byte `json:"ciphertext"`
	Timestamp  int64  `json:"timestamp"`
}

// GroupMembers announces the member list for a group.
type GroupMembers struct {
	GroupID string   `json:"group_id"`
	Members []string `json:"members"`
}

// MaxMessageSize is the maximum size of a single framed message (10MB).
const MaxMessageSize = 10 * 1024 * 1024

// Wrap creates an Envelope from a typed message.
func Wrap(msgType MessageType, msg any) (*Envelope, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &Envelope{
		Type:    msgType,
		Payload: payload,
	}, nil
}

// Unwrap extracts the payload from an Envelope into the target struct.
func Unwrap[T any](env *Envelope) (*T, error) {
	var t T
	if err := json.Unmarshal(env.Payload, &t); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &t, nil
}

// WriteMessage writes a length-prefixed JSON envelope to w.
// Frame: [4 bytes big-endian length][JSON payload]
func WriteMessage(w io.Writer, env *Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes", len(data))
	}

	// Write length prefix
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("write length: %w", err)
	}

	// Write payload
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

// ReadMessage reads a length-prefixed JSON envelope from r.
func ReadMessage(r io.Reader) (*Envelope, error) {
	// Read length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read payload
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	return &env, nil
}
