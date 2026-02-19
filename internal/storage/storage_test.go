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

func TestFileInfoPersistence(t *testing.T) {
	s := tempStore(t)

	msgs := []storage.StoredMessage{
		{
			ID:      "file-msg-1",
			Sender:  "alice",
			Content: "report.pdf",
			FileInfo: &storage.StoredFileInfo{
				Filename: "report.pdf",
				Size:     2_500_000,
				State:    1, // FileSent
			},
		},
		{
			ID:      "file-msg-2",
			Sender:  "bob",
			Content: "photo.png",
			FileInfo: &storage.StoredFileInfo{
				Filename: "photo.png",
				Size:     500_000,
				State:    3, // FileReceived
				Path:     "/home/user/Downloads/tailchat/photo.png",
			},
		},
		{
			ID:      "file-msg-3",
			Sender:  "alice",
			Content: "broken.zip",
			FileInfo: &storage.StoredFileInfo{
				Filename: "broken.zip",
				Size:     100,
				State:    2, // FileFailed
				Error:    "peer offline",
			},
		},
		{
			ID:      "regular-msg",
			Sender:  "alice",
			Content: "no file here",
			// FileInfo is nil
		},
	}

	if err := s.SaveMessages("alice", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadMessages("alice")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(loaded))
	}

	// File msg 1: sent
	if loaded[0].FileInfo == nil {
		t.Fatal("expected FileInfo on msg 1")
	}
	if loaded[0].FileInfo.Filename != "report.pdf" {
		t.Errorf("expected 'report.pdf', got %q", loaded[0].FileInfo.Filename)
	}
	if loaded[0].FileInfo.Size != 2_500_000 {
		t.Errorf("expected 2500000, got %d", loaded[0].FileInfo.Size)
	}
	if loaded[0].FileInfo.State != 1 {
		t.Errorf("expected state 1, got %d", loaded[0].FileInfo.State)
	}

	// File msg 2: received with path
	if loaded[1].FileInfo == nil {
		t.Fatal("expected FileInfo on msg 2")
	}
	if loaded[1].FileInfo.Path != "/home/user/Downloads/tailchat/photo.png" {
		t.Errorf("expected path, got %q", loaded[1].FileInfo.Path)
	}

	// File msg 3: failed with error
	if loaded[2].FileInfo == nil {
		t.Fatal("expected FileInfo on msg 3")
	}
	if loaded[2].FileInfo.Error != "peer offline" {
		t.Errorf("expected 'peer offline', got %q", loaded[2].FileInfo.Error)
	}

	// Regular msg: no FileInfo
	if loaded[3].FileInfo != nil {
		t.Error("expected nil FileInfo on regular message")
	}
}

func TestFileInfoSearch(t *testing.T) {
	s := tempStore(t)

	msgs := []storage.StoredMessage{
		{
			ID:      "file-1",
			Sender:  "alice",
			Content: "report.pdf",
			FileInfo: &storage.StoredFileInfo{
				Filename: "report.pdf",
				Size:     1000,
				State:    1,
			},
		},
		{
			ID:      "msg-1",
			Sender:  "alice",
			Content: "check the report",
		},
	}

	s.SaveMessages("alice", msgs)

	// Search should find both by "report"
	results, err := s.Search("report")
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
