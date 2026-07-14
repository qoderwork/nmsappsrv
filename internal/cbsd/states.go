package cbsd

import (
	"strconv"
	"strings"
	"time"
)

// CBSD SAS operation-state values (mirrors Java cbsd_info.operation_state).
const (
	OpStateUnregistered = "UNREGISTERED"
	OpStateRegistered   = "REGISTERED"
	OpStateGranted      = "GRANTED"
	OpStateAuthorized   = "AUTHORIZED"
	OpStateSuspended    = "SUSPENDED"
)

// transitionPlan is the pure result of decideTransition: what the next state
// should be and which side effects to apply. It is kept free of I/O so it can
// be unit-tested without a database or SAS connection.
type transitionPlan struct {
	newState      string
	clearGrant    bool
	clearTransmit bool
	logType       string
	logStatus     string
	reGrant       bool
	reHeartbeat   bool
}

// decideTransition computes the next operation-state and side effects for a
// CBSD based on its current state and grant/transmit expire times. This mirrors
// Java OperationStateMaintainThread:
//   - grantExpireTime passed => revert to REGISTERED, drop grant, re-register+grant.
//   - transmitExpireTime passed (while AUTHORIZED) => revert to GRANTED, re-heartbeat.
//
// A zero-valued plan means "no transition required".
func decideTransition(c *CbsdInfo, now time.Time) transitionPlan {
	if c == nil || c.OperationState == nil {
		return transitionPlan{}
	}
	state := *c.OperationState

	// Grant expiry: applies to GRANTED and AUTHORIZED.
	if (state == OpStateGranted || state == OpStateAuthorized) && c.GrantExpireTime != nil && *c.GrantExpireTime != "" {
		if gt, ok := parseSasTime(*c.GrantExpireTime); ok && now.After(gt) {
			return transitionPlan{
				newState:      OpStateRegistered,
				clearGrant:    true,
				clearTransmit: true,
				logType:       "grant",
				logStatus:     "expired",
				reGrant:       true,
			}
		}
	}

	// Transmit expiry: applies while AUTHORIZED (grant still valid).
	if state == OpStateAuthorized && c.TransmitExpireTime != nil && *c.TransmitExpireTime != "" {
		if tt, ok := parseSasTime(*c.TransmitExpireTime); ok && now.After(tt) {
			return transitionPlan{
				newState:      OpStateGranted,
				clearTransmit: true,
				logType:       "heartbeat",
				logStatus:     "transmit_expired",
				reHeartbeat:   true,
			}
		}
	}

	return transitionPlan{}
}

// parseSasTime parses a SAS timestamp stored in cbsd_info. The column is a
// varchar, so values may arrive as RFC3339, a SQL-style datetime, or a Unix
// epoch (seconds or milliseconds).
func parseSasTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e12 { // milliseconds
			return time.Unix(0, n*int64(time.Millisecond)).UTC(), true
		}
		return time.Unix(n, 0).UTC(), true
	}
	return time.Time{}, false
}

// sasTimeString normalises a SAS response value into a storable string
// (RFC3339). Returns "" when the value is absent or unrecognised.
func sasTimeString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	if f, ok := v.(float64); ok {
		return time.Unix(int64(f), 0).UTC().Format(time.RFC3339)
	}
	return ""
}

// sasResponseCode extracts the CBSD-SAS response code from a SAS response
// envelope. Envelope: {"response":[{"response":{"responseCode":N}, ...}]}.
// When the envelope is absent (e.g. a thin wrapper), it defaults to 0 so a
// successful HTTP 200 is treated as success.
func sasResponseCode(result map[string]interface{}) int {
	resp, ok := result["response"]
	if !ok {
		return 0
	}
	arr, ok := resp.([]interface{})
	if !ok || len(arr) == 0 {
		return 0
	}
	first, ok := arr[0].(map[string]interface{})
	if !ok {
		return 0
	}
	inner, ok := first["response"].(map[string]interface{})
	if !ok {
		return 0
	}
	if code, ok := inner["responseCode"].(float64); ok {
		return int(code)
	}
	return 0
}
