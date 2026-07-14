package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHub_SendToUser verifies directed delivery: only clients authenticated as
// the target user receive the message; other users and anonymous clients do not.
func TestHub_SendToUser(t *testing.T) {
	hub := NewHub()

	alice1 := &Client{hub: hub, send: make(chan []byte, 4), id: "alice1", username: "alice", topics: map[string]bool{}}
	alice2 := &Client{hub: hub, send: make(chan []byte, 4), id: "alice2", username: "alice", topics: map[string]bool{}}
	bob := &Client{hub: hub, send: make(chan []byte, 4), id: "bob", username: "bob", topics: map[string]bool{}}
	anon := &Client{hub: hub, send: make(chan []byte, 4), id: "anon", username: "", topics: map[string]bool{}}

	hub.mu.Lock()
	hub.clients[alice1] = true
	hub.clients[alice2] = true
	hub.clients[bob] = true
	hub.clients[anon] = true
	hub.mu.Unlock()

	msg := []byte(`{"hello":"alice"}`)
	hub.SendToUser("alice", msg)

	// Both of alice's connections receive the message.
	select {
	case got := <-alice1.send:
		assert.Equal(t, msg, got)
	default:
		t.Fatal("alice connection alice1 did not receive per-user message")
	}
	select {
	case got := <-alice2.send:
		assert.Equal(t, msg, got)
	default:
		t.Fatal("alice connection alice2 did not receive per-user message")
	}

	// bob and anon must NOT receive alice's message.
	select {
	case got := <-bob.send:
		t.Fatalf("bob unexpectedly received alice's message: %s", got)
	case <-time.After(100 * time.Millisecond):
		// expected: no message for bob
	}
	select {
	case got := <-anon.send:
		t.Fatalf("anonymous client unexpectedly received alice's message: %s", got)
	case <-time.After(100 * time.Millisecond):
		// expected: no message for anonymous
	}
}

// TestHub_SendToUser_EmptyUsernameIsNoop ensures targeting an empty (anonymous)
// username drops the message instead of broadcasting or panicking.
func TestHub_SendToUser_EmptyUsernameIsNoop(t *testing.T) {
	hub := NewHub()
	c := &Client{hub: hub, send: make(chan []byte, 4), id: "c1", username: "carol", topics: map[string]bool{}}
	hub.mu.Lock()
	hub.clients[c] = true
	hub.mu.Unlock()

	hub.SendToUser("", []byte("should not be delivered"))

	select {
	case got := <-c.send:
		t.Fatalf("empty-username SendToUser unexpectedly delivered: %s", got)
	case <-time.After(100 * time.Millisecond):
		// expected: nothing delivered
	}
}

// TestHub_SendToUser_NoConnectedClient drops the message when no client for the
// user is connected (no panic, no deadlock).
func TestHub_SendToUser_NoConnectedClient(t *testing.T) {
	hub := NewHub()
	hub.SendToUser("ghost", []byte("nobody home"))
}
