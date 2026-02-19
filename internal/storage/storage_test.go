package storage_test

import (
	"os"
	"testing"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/storage"
)

func tempStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.New(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return s
}

func TestSaveAndLoadMessages(t *testing.T) {
	s := tempStore(t)

	msgs := []storage.StoredMessage{
		{
			ID:        "msg-1",
			Sender:    "alice",
			Content:   "hello",
			Timestamp: time.Now().Truncate(time.Millisecond),
			IsOwn:     false,
			Delivered: true,
		},
		{
			ID:        "msg-2",
			Sender:    "bob",
			Content:   "hi there",
			Timestamp: time.Now().Add(time.Second).Truncate(time.Millisecond),
			IsOwn:     true,
			Delivered: true,
			Read:      true,
		},
	}

	if err := s.SaveMessages("alice", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadMessages("alice")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(loaded))
	}
	if loaded[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", loaded[0].Content)
	}
	if loaded[1].Read != true {
		t.Error("expected msg-2 to be read")
	}
}

func TestLoadMessages_NotFound(t *testing.T) {
	s := tempStore(t)

	msgs, err := s.LoadMessages("nonexistent")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if msgs != nil {
		t.Fatalf("expected nil messages, got %d", len(msgs))
	}
}

func TestSaveAndLoadGroups(t *testing.T) {
	s := tempStore(t)

	groups := []storage.StoredGroup{
		{ID: "g1", Name: "Test Group", Members: []string{"alice", "bob", "charlie"}},
	}

	if err := s.SaveGroups(groups); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadGroups()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 group, got %d", len(loaded))
	}
	if loaded[0].Name != "Test Group" {
		t.Errorf("expected 'Test Group', got %q", loaded[0].Name)
	}
	if len(loaded[0].Members) != 3 {
		t.Errorf("expected 3 members, got %d", len(loaded[0].Members))
	}
}

func TestSearch(t *testing.T) {
	s := tempStore(t)

	msgs1 := []storage.StoredMessage{
		{ID: "1", Sender: "alice", Content: "hello world", Timestamp: time.Now()},
		{ID: "2", Sender: "alice", Content: "goodbye", Timestamp: time.Now()},
	}
	msgs2 := []storage.StoredMessage{
		{ID: "3", Sender: "bob", Content: "hello bob", Timestamp: time.Now()},
	}

	s.SaveMessages("alice", msgs1)
	s.SaveMessages("bob", msgs2)

	results, err := s.Search("hello")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	total := 0
	for _, msgs := range results {
		total += len(msgs)
	}
	if total != 2 {
		t.Fatalf("expected 2 results, got %d", total)
	}

	// Search for something that doesn't exist
	results, err = s.Search("xyznotfound")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	total = 0
	for _, msgs := range results {
		total += len(msgs)
	}
	if total != 0 {
		t.Fatalf("expected 0 results, got %d", total)
	}
}

func TestReactionsPersistence(t *testing.T) {
	s := tempStore(t)

	msgs := []storage.StoredMessage{
		{
			ID:      "msg-1",
			Sender:  "alice",
			Content: "nice!",
			Reactions: []storage.StoredReaction{
				{Emoji: "\U0001f44d", Sender: "bob"},
				{Emoji: "\u2764\ufe0f", Sender: "charlie"},
			},
		},
	}

	if err := s.SaveMessages("alice", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadMessages("alice")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded[0].Reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %d", len(loaded[0].Reactions))
	}
	if loaded[0].Reactions[0].Emoji != "\U0001f44d" {
		t.Errorf("wrong emoji: %q", loaded[0].Reactions[0].Emoji)
	}
}

func TestNewStore_DefaultDir(t *testing.T) {
	// Test that New("") uses the default directory without error
	// We can't easily test the actual path without modifying HOME,
	// so just ensure it creates without panic
	s, err := storage.New("")
	if err != nil {
		t.Fatalf("create default store: %v", err)
	}
	// Clean up the default directory
	home, _ := os.UserHomeDir()
	_ = s
	_ = home
}

func TestChatKeyToFilename(t *testing.T) {
	s := tempStore(t)

	// Group chat key uses colon, should be safely stored
	msgs := []storage.StoredMessage{
		{ID: "1", Sender: "alice", Content: "group msg"},
	}
	if err := s.SaveMessages("group:abc-123", msgs); err != nil {
		t.Fatalf("save group messages: %v", err)
	}

	loaded, err := s.LoadMessages("group:abc-123")
	if err != nil {
		t.Fatalf("load group messages: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1, got %d", len(loaded))
	}
}
