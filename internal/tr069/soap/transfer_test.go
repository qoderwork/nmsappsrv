package soap

import (
	"strings"
	"testing"
)

const reportTransmissionProgressSample = `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-0">
<soap:Header><cwmp:ID>1</cwmp:ID></soap:Header>
<soap:Body>
<cwmp:ReportTransmissionProgress>
<CommandKey>log_42</CommandKey>
<ProgressPercentage>37</ProgressPercentage>
</cwmp:ReportTransmissionProgress>
</soap:Body>
</soap:Envelope>`

const fragmentTransferCompleteSample = `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-0">
<soap:Header><cwmp:ID>2</cwmp:ID></soap:Header>
<soap:Body>
<cwmp:FragmentTransferComplete>
<CommandKey>log_42</CommandKey>
<FaultStruct>
<FaultCode>0</FaultCode>
<FaultString></FaultString>
</FaultStruct>
<TargetFileName>vendor.log</TargetFileName>
<FileType>4 Vendor Log File</FileType>
</cwmp:FragmentTransferComplete>
</soap:Body>
</soap:Envelope>`

const autonomousFragmentTransferCompleteSample = `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cwmp="urn:dslforum-org:cwmp-1-0">
<soap:Header><cwmp:ID>3</cwmp:ID></soap:Header>
<soap:Body>
<cwmp:AutonomousFragmentTransferComplete>
<CommandKey>log_99</CommandKey>
<FaultStruct>
<FaultCode>9002</FaultCode>
<FaultString>Internal error</FaultString>
</FaultStruct>
<TargetFileName>vendor.pcap</TargetFileName>
<FileType>5 Vendor Capture File</FileType>
</cwmp:AutonomousFragmentTransferComplete>
</soap:Body>
</soap:Envelope>`

func TestParseReportTransmissionProgress(t *testing.T) {
	rtp, err := ParseReportTransmissionProgress(reportTransmissionProgressSample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rtp.Header.ID != "1" {
		t.Errorf("expected header ID '1', got %q", rtp.Header.ID)
	}
	if rtp.CommandKey != "log_42" {
		t.Errorf("expected CommandKey 'log_42', got %q", rtp.CommandKey)
	}
	if rtp.ProgressPercentage != "37" {
		t.Errorf("expected ProgressPercentage '37', got %q", rtp.ProgressPercentage)
	}
}

func TestParseFragmentTransferComplete(t *testing.T) {
	ft, err := ParseFragmentTransferComplete(fragmentTransferCompleteSample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ft.CommandKey != "log_42" {
		t.Errorf("expected CommandKey 'log_42', got %q", ft.CommandKey)
	}
	if ft.FaultCode != 0 {
		t.Errorf("expected FaultCode 0, got %d", ft.FaultCode)
	}
	if ft.TargetFileName != "vendor.log" {
		t.Errorf("expected TargetFileName 'vendor.log', got %q", ft.TargetFileName)
	}
	if ft.FileType != "4 Vendor Log File" {
		t.Errorf("expected FileType '4 Vendor Log File', got %q", ft.FileType)
	}
}

func TestParseAutonomousFragmentTransferComplete(t *testing.T) {
	a, err := ParseAutonomousFragmentTransferComplete(autonomousFragmentTransferCompleteSample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.CommandKey != "log_99" {
		t.Errorf("expected CommandKey 'log_99', got %q", a.CommandKey)
	}
	if a.FaultCode != 9002 {
		t.Errorf("expected FaultCode 9002, got %d", a.FaultCode)
	}
	if a.FaultString != "Internal error" {
		t.Errorf("expected FaultString 'Internal error', got %q", a.FaultString)
	}
	if a.FileType != "5 Vendor Capture File" {
		t.Errorf("expected FileType '5 Vendor Capture File', got %q", a.FileType)
	}
}

func TestDetectMessageType_ReportTransmissionProgress(t *testing.T) {
	if got := DetectMessageType(reportTransmissionProgressSample); got != MsgReportTransmissionProgress {
		t.Errorf("expected MsgReportTransmissionProgress (%d), got %d", MsgReportTransmissionProgress, got)
	}
}

func TestDetectMessageType_AutonomousFragmentTransferComplete(t *testing.T) {
	if got := DetectMessageType(autonomousFragmentTransferCompleteSample); got != MsgAutonomousFragmentTransferComplete {
		t.Errorf("expected MsgAutonomousFragmentTransferComplete (%d), got %d", MsgAutonomousFragmentTransferComplete, got)
	}
}

func TestBuildReportTransmissionProgressResponse(t *testing.T) {
	out := BuildReportTransmissionProgressResponse("abc-123")
	if !strings.Contains(out, "ReportTransmissionProgressResponse") {
		t.Errorf("response missing ReportTransmissionProgressResponse: %s", out)
	}
	if !strings.Contains(out, "abc-123") {
		t.Errorf("response missing header ID: %s", out)
	}
}
