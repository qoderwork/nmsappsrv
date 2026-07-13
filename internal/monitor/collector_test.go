package monitor

import (
	"testing"

	"nmsappsrv/internal/tr069/soap"
)

func TestMatchParam(t *testing.T) {
	pathToPID := map[string]string{
		"Device.foo":    "pid-foo",
		"Device.boats.": "pid-boats",
	}
	cases := []struct {
		name string
		want string
		ok   bool
	}{
		{"Device.foo", "pid-foo", true},
		{"Device.boats.1", "pid-boats", true},  // multi-instance expansion
		{"Device.boats.12", "pid-boats", true}, // multi-instance expansion
		{"Device.unknown", "", false},
		{"Device.foo.extra", "", false}, // not a prefix match
	}
	for _, c := range cases {
		got, ok := matchParam(c.name, pathToPID)
		if ok != c.ok || got != c.want {
			t.Errorf("matchParam(%q) = (%q,%v), want (%q,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestAggregateMonitorValues(t *testing.T) {
	pathToPID := map[string]string{
		"Device.cpu":    "pid-cpu",
		"Device.boats.": "pid-boats",
	}

	values := []soap.ParameterValueStruct{
		{Name: "Device.cpu", Value: "50"},
		{Name: "Device.boats.1", Value: "10"},
		{Name: "Device.boats.2", Value: "20"},
		{Name: "Device.boats.3", Value: "30"},
		{Name: "Device.unknown", Value: "99"}, // unmapped -> skipped
		{Name: "Device.boats.4", Value: "abc"}, // non-numeric -> skipped
	}

	agg := aggregateMonitorValues(values, pathToPID)

	if len(agg) != 2 {
		t.Fatalf("expected 2 aggregated params, got %d: %v", len(agg), agg)
	}
	// cpu: single value -> 50
	if cpu, ok := agg["pid-cpu"]; !ok || cpu != 50 {
		t.Errorf("pid-cpu = %v (ok=%v), want 50", cpu, ok)
	}
	// boats: (10+20+30)/3 = 20 (multi-instance average, mirrors Java {i} expansion)
	if boats, ok := agg["pid-boats"]; !ok || boats != 20 {
		t.Errorf("pid-boats = %v (ok=%v), want 20", boats, ok)
	}
}

func TestParseIDLists(t *testing.T) {
	s := strPtr(`[1,2,3]`)
	if got := parseInt64List(s); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("parseInt64List = %v, want [1,2,3]", got)
	}
	gs := strPtr(`["g1","g2"]`)
	if got := parseStringList(gs); len(got) != 2 || got[0] != "g1" {
		t.Errorf("parseStringList = %v, want [g1,g2]", got)
	}
	if got := parseInt64List(nil); got != nil {
		t.Errorf("parseInt64List(nil) = %v, want nil", got)
	}
}

func strPtr(s string) *string { return &s }
