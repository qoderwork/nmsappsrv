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

// TestHub_Heartbeat_OnlineStatus verifies that UpdateHeartbeat + IsUserOnline
// correctly track user online state (mirrors Java lastHeartbeatTime check).
func TestHub_Heartbeat_OnlineStatus(t *testing.T) {
	hub := NewHub()

	// Before any heartbeat, user is offline.
	if hub.IsUserOnline("alice") {
		t.Fatal("expected alice to be offline before heartbeat")
	}

	// After heartbeat, user is online.
	hub.UpdateHeartbeat("alice")
	if !hub.IsUserOnline("alice") {
		t.Fatal("expected alice to be online after heartbeat")
	}

	// Empty username is always offline.
	if hub.IsUserOnline("") {
		t.Fatal("expected empty username to be offline")
	}
}

// TestHub_Heartbeat_ClearOnDisconnect verifies that heartbeat is cleared when
// the last connection for a user is removed via the unregister channel.
func TestHub_Heartbeat_ClearOnDisconnect(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	alice := &Client{hub: hub, send: make(chan []byte, 4), id: "alice1", username: "alice", topics: map[string]bool{}}
	hub.register <- alice

	// Wait for register to process
	time.Sleep(50 * time.Millisecond)
	if !hub.IsUserOnline("alice") {
		t.Fatal("expected alice to be online after register")
	}

	// Disconnect — heartbeat should be cleared.
	hub.unregister <- alice
	time.Sleep(50 * time.Millisecond)
	if hub.IsUserOnline("alice") {
		t.Fatal("expected alice to be offline after disconnect")
	}
}

// TestHub_Heartbeat_MultipleConnections verifies that heartbeat is only cleared
// when the LAST connection for a user is removed (multi-tab scenario).
func TestHub_Heartbeat_MultipleConnections(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	alice1 := &Client{hub: hub, send: make(chan []byte, 4), id: "alice1", username: "alice", topics: map[string]bool{}}
	alice2 := &Client{hub: hub, send: make(chan []byte, 4), id: "alice2", username: "alice", topics: map[string]bool{}}
	hub.register <- alice1
	hub.register <- alice2
	time.Sleep(50 * time.Millisecond)

	if !hub.IsUserOnline("alice") {
		t.Fatal("expected alice to be online with 2 connections")
	}

	// Close first connection — alice should still be online (second tab open).
	hub.unregister <- alice1
	time.Sleep(50 * time.Millisecond)
	if !hub.IsUserOnline("alice") {
		t.Fatal("expected alice to still be online after closing one of two connections")
	}

	// Close second connection — alice should now be offline.
	hub.unregister <- alice2
	time.Sleep(50 * time.Millisecond)
	if hub.IsUserOnline("alice") {
		t.Fatal("expected alice to be offline after closing all connections")
	}
}
