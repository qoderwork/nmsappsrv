package tcpdump

import (
	"testing"
	"time"
)

func TestBuildCaptureFileName(t *testing.T) {
	// Pinned timestamps so the test is deterministic and mirrors Java's
	// yyyyMMddHHmmss formatting.
	start := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 13, 12, 0, 30, 0, time.UTC)

	got := buildCaptureFileName("comm", start, end)
	want := "comm_20260713120000_20260713120030.pcap"
	if got != want {
		t.Fatalf("buildCaptureFileName = %q, want %q", got, want)
	}

	// Container is used verbatim as the prefix (api/comm/core), matching Java.
	if got := buildCaptureFileName("api", start, end); got != "api_20260713120000_20260713120030.pcap" {
		t.Fatalf("api prefix = %q", got)
	}
	if got := buildCaptureFileName("core", start, end); got != "core_20260713120000_20260713120030.pcap" {
		t.Fatalf("core prefix = %q", got)
	}
}

func TestIsValidFileName(t *testing.T) {
	valid := []string{
		"comm_20260713120000_20260713120030.pcap",
		"api_20260101000000_20260101000010.pcap",
		"core.pcap",
		"file-name.pcap",
	}
	for _, n := range valid {
		if !isValidFileName(n) {
			t.Errorf("isValidFileName(%q) = false, want true", n)
		}
	}

	invalid := []string{
		"",
		"   ",
		"../etc/passwd",
		"../../etc/passwd",
		"comm/../../etc/passwd",
		"foo\\bar.pcap",
		"foo/bar.pcap",
		"..",
		"./comm_1.pcap",
	}
	for _, n := range invalid {
		if isValidFileName(n) {
			t.Errorf("isValidFileName(%q) = true, want false", n)
		}
	}
}
