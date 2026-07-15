package captcha

import (
	"context"
	"strings"
	"testing"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/alicebob/miniredis/v2"
)

func newTestManager(t *testing.T) (*Manager, *miniredis.Miniredis, *goredis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewManager(rdb, 4), mr, rdb
}

func TestGenerateAndVerify(t *testing.T) {
	mgr, _, rdb := newTestManager(t)
	ctx := context.Background()

	id, b64, err := mgr.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if id == "" {
		t.Fatal("empty captcha id")
	}
	if !strings.HasPrefix(b64, "data:image/png;base64,") {
		t.Fatalf("imageBase64 missing data URI prefix: %q", b64)
	}

	// Answer is stored server-side under captcha_code_<id>.
	answer, err := rdb.Get(ctx, captchaKeyPrefix+id).Result()
	if err != nil {
		t.Fatalf("answer not stored: %v", err)
	}
	if answer == "" {
		t.Fatal("empty stored answer")
	}

	if !mgr.Verify(ctx, id, answer) {
		t.Fatal("Verify should succeed with correct answer")
	}
	if mgr.Verify(ctx, id, answer) {
		t.Fatal("Verify should fail after consumption (no replay)")
	}
	if mgr.Verify(ctx, id, "0000") {
		t.Fatal("Verify should fail with wrong answer")
	}
	if mgr.Verify(ctx, "nope", answer) {
		t.Fatal("Verify should fail with unknown id")
	}
	if mgr.Verify(ctx, "", "") {
		t.Fatal("Verify should fail on empty input")
	}
}

func TestGuardUIPTriggerAndIndependence(t *testing.T) {
	mgr, _, _ := newTestManager(t)

	if mgr.IsRequired("alice", "1.2.3.4") {
		t.Fatal("should not require captcha initially")
	}

	// 3 username+IP failures -> required for that combo (threshold 3).
	for i := 0; i < 3; i++ {
		mgr.OnFailure("alice", "1.2.3.4")
	}
	if !mgr.IsRequired("alice", "1.2.3.4") {
		t.Fatal("alice@1.2.3.4 should require captcha after 3 fails")
	}
	// Dimension independence: same user, different IP not required yet
	// (username-wide threshold is 5).
	if mgr.IsRequired("alice", "9.9.9.9") {
		t.Fatal("alice@9.9.9.9 should NOT require yet (only uip crossed)")
	}
	// Different user, same IP not required yet (IP-wide threshold is 10).
	if mgr.IsRequired("bob", "1.2.3.4") {
		t.Fatal("bob@1.2.3.4 should NOT require yet")
	}

	mgr.OnSuccess("alice", "1.2.3.4")
	if mgr.IsRequired("alice", "1.2.3.4") {
		t.Fatal("captcha should be cleared after OnSuccess")
	}
}

func TestGuardUserWide(t *testing.T) {
	mgr, _, _ := newTestManager(t)

	for i := 0; i < 5; i++ {
		mgr.OnFailure("bob", "5.5.5.5")
	}
	if !mgr.IsRequired("bob", "5.5.5.5") {
		t.Fatal("bob should require after 5 user-wide fails")
	}
	// Username-wide marker applies across IPs.
	if !mgr.IsRequired("bob", "6.6.6.6") {
		t.Fatal("bob@6.6.6.6 should require (user-wide marker)")
	}
	// Another user on the same IP is not required yet (IP threshold 10).
	if mgr.IsRequired("carol", "5.5.5.5") {
		t.Fatal("carol@5.5.5.5 should NOT require yet")
	}
}

func TestGuardIPWide(t *testing.T) {
	mgr, _, _ := newTestManager(t)

	for i := 0; i < 10; i++ {
		mgr.OnFailure("u1", "7.7.7.7")
	}
	if !mgr.IsRequired("u1", "7.7.7.7") {
		t.Fatal("should require after 10 IP-wide fails")
	}
	if !mgr.IsRequired("anyone", "7.7.7.7") {
		t.Fatal("any user@7.7.7.7 should require (IP-wide marker)")
	}
}

func TestGuardMarkerExpiry(t *testing.T) {
	mgr, mr, _ := newTestManager(t)

	for i := 0; i < 3; i++ {
		mgr.OnFailure("alice", "1.2.3.4")
	}
	if !mgr.IsRequired("alice", "1.2.3.4") {
		t.Fatal("should require before expiry")
	}
	mr.FastForward(reqTTL + time.Second)
	if mgr.IsRequired("alice", "1.2.3.4") {
		t.Fatal("captcha requirement should expire after reqTTL")
	}
}

// TestFailOpenOnRedisDown ensures the guard never blocks login or panics when
// Redis is unreachable.
func TestFailOpenOnRedisDown(t *testing.T) {
	rdb := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 10 * time.Millisecond})
	defer rdb.Close()
	mgr := NewManager(rdb, 4)

	if mgr.IsRequired("x", "y") {
		t.Fatal("should be fail-open (false) when redis down")
	}
	mgr.OnFailure("x", "y") // must not panic
	mgr.OnSuccess("x", "y") // must not panic
	if _, _, err := mgr.Generate(context.Background()); err == nil {
		t.Fatal("Generate should error when redis down")
	}
}
