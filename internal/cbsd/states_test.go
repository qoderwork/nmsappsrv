package cbsd

import (
	"testing"
	"time"
)

// ----- parseSasTime -----------------------------------------------------

func TestParseSasTime(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"rfc3339", "2026-01-02T15:04:05Z", true},
		{"rfc3339 with tz", "2026-01-02T15:04:05+08:00", true},
		{"sql datetime", "2026-01-02 15:04:05", true},
		{"sql datetime with tz", "2026-01-02 15:04:05+08:00", true},
		{"epoch seconds", "1700000000", true},
		{"epoch millis", "1700000000000", true},
		{"empty", "", false},
		{"garbage", "not-a-time", false},
		{"whitespace wraps valid", "  2026-01-02T15:04:05Z  ", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseSasTime(c.in)
			if ok != c.want {
				t.Fatalf("parseSasTime(%q) ok=%v want %v", c.in, ok, c.want)
			}
			if ok && got.IsZero() {
				t.Fatalf("parseSasTime(%q) returned zero time", c.in)
			}
		})
	}
}

// ----- sasTimeString ----------------------------------------------------

func TestSasTimeString(t *testing.T) {
	if got := sasTimeString(nil); got != "" {
		t.Fatalf("nil -> %q", got)
	}
	if got := sasTimeString("  2026-01-02T15:04:05Z  "); got != "2026-01-02T15:04:05Z" {
		t.Fatalf("string trim -> %q", got)
	}
	got := sasTimeString(float64(1700000000))
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("float64 not RFC3339: %q (%v)", got, err)
	}
}

// ----- sasResponseCode --------------------------------------------------

func TestSasResponseCode(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]interface{}
		want int
	}{
		{"code 0", map[string]interface{}{"response": []interface{}{map[string]interface{}{"response": map[string]interface{}{"responseCode": float64(0)}}}}, 0},
		{"code 501", map[string]interface{}{"response": []interface{}{map[string]interface{}{"response": map[string]interface{}{"responseCode": float64(501)}}}}, 501},
		{"code 500", map[string]interface{}{"response": []interface{}{map[string]interface{}{"response": map[string]interface{}{"responseCode": float64(500)}}}}, 500},
		{"missing response", map[string]interface{}{}, 0},
		{"empty response array", map[string]interface{}{"response": []interface{}{}}, 0},
		{"missing inner response", map[string]interface{}{"response": []interface{}{map[string]interface{}{}}}, 0},
		{"non-float code", map[string]interface{}{"response": []interface{}{map[string]interface{}{"response": map[string]interface{}{"responseCode": "0"}}}}, 0},
		{"nil map", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sasResponseCode(c.in); got != c.want {
				t.Fatalf("sasResponseCode = %d want %d", got, c.want)
			}
		})
	}
}

// ----- decideTransition -------------------------------------------------

func TestDecideTransition(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour).Format(time.RFC3339)
	future := now.Add(time.Hour).Format(time.RFC3339)

	t.Run("nil cbsd", func(t *testing.T) {
		if p := decideTransition(nil, now); p.newState != "" {
			t.Fatalf("nil -> %+v", p)
		}
	})
	t.Run("nil state", func(t *testing.T) {
		if p := decideTransition(&CbsdInfo{}, now); p.newState != "" {
			t.Fatalf("nil state -> %+v", p)
		}
	})
	t.Run("granted grant expired", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateGranted), GrantExpireTime: strPtr(past), TransmitExpireTime: strPtr(future)}
		p := decideTransition(c, now)
		if p.newState != OpStateRegistered || !p.clearGrant || !p.clearTransmit || !p.reGrant {
			t.Fatalf("granted/expired -> %+v", p)
		}
		if p.logType != "grant" || p.logStatus != "expired" {
			t.Fatalf("granted/expired log -> %+v", p)
		}
	})
	t.Run("authorized grant expired", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateAuthorized), GrantExpireTime: strPtr(past), TransmitExpireTime: strPtr(future)}
		p := decideTransition(c, now)
		if p.newState != OpStateRegistered || !p.reGrant || !p.clearGrant || !p.clearTransmit {
			t.Fatalf("authorized/grant-expired -> %+v", p)
		}
	})
	t.Run("authorized transmit expired", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateAuthorized), GrantExpireTime: strPtr(future), TransmitExpireTime: strPtr(past)}
		p := decideTransition(c, now)
		if p.newState != OpStateGranted || !p.clearTransmit || !p.reHeartbeat {
			t.Fatalf("authorized/transmit-expired -> %+v", p)
		}
		if p.logType != "heartbeat" || p.logStatus != "transmit_expired" {
			t.Fatalf("authorized/transmit-expired log -> %+v", p)
		}
	})
	t.Run("granted grant future", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateGranted), GrantExpireTime: strPtr(future), TransmitExpireTime: strPtr(future)}
		if p := decideTransition(c, now); p.newState != "" {
			t.Fatalf("granted/future -> %+v", p)
		}
	})
	t.Run("authorized both future", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateAuthorized), GrantExpireTime: strPtr(future), TransmitExpireTime: strPtr(future)}
		if p := decideTransition(c, now); p.newState != "" {
			t.Fatalf("authorized/future -> %+v", p)
		}
	})
	t.Run("registered untouched", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateRegistered), GrantExpireTime: strPtr(past), TransmitExpireTime: strPtr(past)}
		if p := decideTransition(c, now); p.newState != "" {
			t.Fatalf("registered -> %+v", p)
		}
	})
	t.Run("empty expire string no transition", func(t *testing.T) {
		c := &CbsdInfo{OperationState: strPtr(OpStateGranted), GrantExpireTime: strPtr("")}
		if p := decideTransition(c, now); p.newState != "" {
			t.Fatalf("empty grant expire -> %+v", p)
		}
	})
	// SAS timestamps are second-resolution, while `now` carries sub-second
	// precision. A grant whose expire *second* has already elapsed must
	// transition, even if `now` is only milliseconds past that second.
	t.Run("boundary expire second elapsed -> transition", func(t *testing.T) {
		base := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
		now := base.Add(500 * time.Millisecond)
		eq := base.Format(time.RFC3339) // re-parses to `base` (no ns)
		c := &CbsdInfo{OperationState: strPtr(OpStateGranted), GrantExpireTime: strPtr(eq)}
		if p := decideTransition(c, now); p.newState != OpStateRegistered {
			t.Fatalf("boundary past-second -> %+v", p)
		}
	})
	t.Run("boundary expire next second -> no transition", func(t *testing.T) {
		base := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
		now := base.Add(500 * time.Millisecond)
		next := base.Add(time.Second).Format(time.RFC3339)
		c := &CbsdInfo{OperationState: strPtr(OpStateGranted), GrantExpireTime: strPtr(next)}
		if p := decideTransition(c, now); p.newState != "" {
			t.Fatalf("boundary future-second -> %+v", p)
		}
	})
}
