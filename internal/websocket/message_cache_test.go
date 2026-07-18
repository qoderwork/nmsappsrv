package websocket

import (
	"testing"
	"time"
)

// TestMessageCache_AddRemove verifies basic add/remove operations.
func TestMessageCache_AddRemove(t *testing.T) {
	cache := NewMessageCache()

	msg := &CachedMessage{
		Data:     "hello",
		Username: "alice",
	}
	cache.Add(msg)

	if cache.Size() != 1 {
		t.Fatalf("expected size 1, got %d", cache.Size())
	}
	if msg.MessageId == "" {
		t.Fatal("expected messageId to be auto-generated")
	}

	cache.Remove(msg.MessageId)
	if cache.Size() != 0 {
		t.Fatalf("expected size 0 after remove, got %d", cache.Size())
	}
}

// TestMessageCache_GetPending_RetryUnder3 verifies that messages retried < 3
// times are returned after 10s.
func TestMessageCache_GetPending_RetryUnder3(t *testing.T) {
	cache := NewMessageCache()

	msg := &CachedMessage{
		Data:         "hello",
		Username:     "alice",
		LastSendTime: time.Now().Add(-11 * time.Second).UnixMilli(),
		RetriedTimes: 0,
		GeneratedTime: time.Now().UnixMilli(),
	}
	cache.Add(msg)

	pending := cache.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
}

// TestMessageCache_GetPending_NotDue verifies that recently sent messages are
// not returned.
func TestMessageCache_GetPending_NotDue(t *testing.T) {
	cache := NewMessageCache()

	msg := &CachedMessage{
		Data:         "hello",
		Username:     "alice",
		LastSendTime: time.Now().Add(-1 * time.Second).UnixMilli(),
		RetriedTimes: 0,
		GeneratedTime: time.Now().UnixMilli(),
	}
	cache.Add(msg)

	pending := cache.GetPending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(pending))
	}
}

// TestMessageCache_GetPending_RetryOver3 verifies that messages retried >= 3
// times use exponential backoff (retriedTimes * 60s).
func TestMessageCache_GetPending_RetryOver3(t *testing.T) {
	cache := NewMessageCache()

	// Retried 3 times, last sent 4 minutes ago (3*60=180s, 240s > 180s → due)
	msg := &CachedMessage{
		Data:         "hello",
		Username:     "alice",
		LastSendTime: time.Now().Add(-4 * time.Minute).UnixMilli(),
		RetriedTimes: 3,
		GeneratedTime: time.Now().UnixMilli(),
	}
	cache.Add(msg)

	pending := cache.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
}

// TestMessageCache_GetExpired verifies that messages older than 24h are expired.
func TestMessageCache_GetExpired(t *testing.T) {
	cache := NewMessageCache()

	// 25 hours old
	msg := &CachedMessage{
		Data:         "hello",
		Username:     "alice",
		LastSendTime: time.Now().UnixMilli(),
		GeneratedTime: time.Now().Add(-25 * time.Hour).UnixMilli(),
	}
	cache.Add(msg)

	expired := cache.GetExpired()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired, got %d", len(expired))
	}
}

// TestHub_SendToUser_WithCache verifies that SendToUser enqueues into the cache
// when one is attached.
func TestHub_SendToUser_WithCache(t *testing.T) {
	hub := NewHub()
	cache := NewMessageCache()
	hub.SetMessageCache(cache)

	hub.SendToUser("alice", []byte("test message"))

	if cache.Size() != 1 {
		t.Fatalf("expected cache size 1, got %d", cache.Size())
	}
}

// TestHub_SendToUser_NoCache verifies that SendToUser falls back to direct
// delivery when no cache is attached.
func TestHub_SendToUser_NoCache(t *testing.T) {
	hub := NewHub()
	// No cache attached — should not panic.
	hub.SendToUser("ghost", []byte("nobody home"))
}

// TestHub_HasUserConnection verifies connection detection.
func TestHub_HasUserConnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	alice := &Client{hub: hub, send: make(chan []byte, 4), id: "alice1", username: "alice", topics: map[string]bool{}}
	hub.register <- alice
	time.Sleep(50 * time.Millisecond)

	if !hub.HasUserConnection("alice") {
		t.Fatal("expected alice to have a connection")
	}
	if hub.HasUserConnection("bob") {
		t.Fatal("expected bob to not have a connection")
	}
}
