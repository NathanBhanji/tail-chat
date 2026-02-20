package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
)

// TestPeerJSONTags verifies discovery.Peer serializes with the expected field names.
func TestPeerJSONTags(t *testing.T) {
	p := discovery.Peer{
		Hostname:    "test-host",
		DNSName:     "test-host.tail123.ts.net",
		TailscaleIP: "100.64.0.1",
		Online:      true,
		OS:          "linux",
		IsSelf:      false,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal Peer: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal Peer: %v", err)
	}

	expected := []string{"hostname", "dnsName", "tailscaleIP", "online", "os", "isSelf"}
	for _, key := range expected {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON field %q in Peer (got keys: %v)", key, keys(m))
		}
	}

	if m["hostname"] != "test-host" {
		t.Errorf("hostname = %v, want test-host", m["hostname"])
	}
	if m["online"] != true {
		t.Errorf("online = %v, want true", m["online"])
	}
}

// TestMessageJSONTags verifies chat.Message serializes with the expected field names.
func TestMessageJSONTags(t *testing.T) {
	msg := chat.Message{
		ID:        "msg-123",
		Sender:    "alice",
		Content:   "hello",
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		IsOwn:     true,
		GroupID:   "grp-456",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal Message: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal Message: %v", err)
	}

	expected := []string{"id", "sender", "content", "timestamp", "isOwn", "groupID"}
	for _, key := range expected {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON field %q in Message (got keys: %v)", key, keys(m))
		}
	}

	if m["sender"] != "alice" {
		t.Errorf("sender = %v, want alice", m["sender"])
	}
	if m["isOwn"] != true {
		t.Errorf("isOwn = %v, want true", m["isOwn"])
	}
}

// TestGroupJSONTags verifies chat.Group serializes with the expected field names.
func TestGroupJSONTags(t *testing.T) {
	g := chat.Group{
		ID:      "grp-789",
		Name:    "test-group",
		Members: []string{"alice", "bob"},
	}

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal Group: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal Group: %v", err)
	}

	expected := []string{"id", "name", "members"}
	for _, key := range expected {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON field %q in Group (got keys: %v)", key, keys(m))
		}
	}

	if m["name"] != "test-group" {
		t.Errorf("name = %v, want test-group", m["name"])
	}
}

// TestGetSelfInfoShape verifies the return shape of GetSelfInfo.
func TestGetSelfInfoShape(t *testing.T) {
	app := &App{selfHost: "my-machine", selfIP: "100.64.0.5"}
	info := app.GetSelfInfo()

	if info["hostname"] != "my-machine" {
		t.Errorf("hostname = %v, want my-machine", info["hostname"])
	}
	if info["ip"] != "100.64.0.5" {
		t.Errorf("ip = %v, want 100.64.0.5", info["ip"])
	}
}

func keys(m map[string]interface{}) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
