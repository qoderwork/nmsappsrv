package northinterfacelog

import "testing"

func TestParseTimeFlex(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"2026-07-15T09:24:55Z", true},
		{"2026-07-15T09:24:55", true},
		{"2026-07-15 09:24:55", true},
		{"2026-07-15", true},
		{"not-a-time", false},
		{"", false},
	}
	for _, c := range cases {
		_, err := parseTimeFlex(c.in)
		got := err == nil
		if got != c.want {
			t.Errorf("parseTimeFlex(%q) ok=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseTimeFlexLayoutsEqual(t *testing.T) {
	// The two near-identical layouts must parse to the same instant.
	a, _ := parseTimeFlex("2026-07-15T09:24:55")
	b, _ := parseTimeFlex("2026-07-15 09:24:55")
	if !a.Equal(b) {
		t.Errorf("layouts disagree: %v vs %v", a, b)
	}
}
