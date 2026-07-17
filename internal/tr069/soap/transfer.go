package soap

import (
	"encoding/xml"
	"fmt"
)

// TransferComplete (CPE -> ACS)
type TransferComplete struct {
	Header       SoapHeader
	CommandKey   string
	FaultCode    int
	FaultString  string
	StartTime    string
	CompleteTime string
}

// ParseTransferComplete parses TransferComplete message from CPE
func ParseTransferComplete(xmlStr string) (*TransferComplete, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var tc transferCompleteXML
	if err := xml.Unmarshal(env.Body.Content, &tc); err != nil {
		return nil, fmt.Errorf("failed to parse TransferComplete: %w", err)
	}

	return &TransferComplete{
		Header: SoapHeader{
			ID: env.Header.ID,
		},
		CommandKey:   tc.CommandKey,
		FaultCode:    tc.FaultCode,
		FaultString:  tc.FaultString,
		StartTime:    tc.StartTime,
		CompleteTime: tc.CompleteTime,
	}, nil
}

// ReportTransmissionProgress (CPE -> ACS) carries transfer progress
// information (CommandKey + ProgressPercentage) for an ongoing Download/Upload.
//
// Mirrors Java NgLogProcessHandler / DeviceLogProgress.
type ReportTransmissionProgress struct {
	Header             SoapHeader
	CommandKey         string
	ProgressPercentage string
}

type reportTransmissionProgressXML struct {
	XMLName             xml.Name `xml:"ReportTransmissionProgress"`
	CommandKey          string   `xml:"CommandKey"`
	ProgressPercentage  string   `xml:"ProgressPercentage"`
}

// ParseReportTransmissionProgress parses a ReportTransmissionProgress message
// from a CPE.
func ParseReportTransmissionProgress(xmlStr string) (*ReportTransmissionProgress, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var rtp reportTransmissionProgressXML
	if err := xml.Unmarshal(env.Body.Content, &rtp); err != nil {
		return nil, fmt.Errorf("failed to parse ReportTransmissionProgress: %w", err)
	}

	return &ReportTransmissionProgress{
		Header: SoapHeader{
			ID: env.Header.ID,
		},
		CommandKey:         rtp.CommandKey,
		ProgressPercentage: rtp.ProgressPercentage,
	}, nil
}

// FragmentTransferComplete (CPE -> ACS) signals a fragment transfer finished.
//
// Mirrors Java FragmentTransferCompleteHandler.
type FragmentTransferComplete struct {
	Header       SoapHeader
	CommandKey   string
	FaultCode    int    // 0 = success
	FaultString  string
	TargetFileName string
	FileType     string
}

type fragmentTransferCompleteXML struct {
	XMLName         xml.Name `xml:"FragmentTransferComplete"`
	CommandKey      string   `xml:"CommandKey"`
	FaultStruct     *faultStructXML `xml:"FaultStruct"`
	TargetFileName  string   `xml:"TargetFileName"`
	FileType        string   `xml:"FileType"`
}

type faultStructXML struct {
	FaultCode   int    `xml:"FaultCode"`
	FaultString string `xml:"FaultString"`
}

// ParseFragmentTransferComplete parses a FragmentTransferComplete message.
func ParseFragmentTransferComplete(xmlStr string) (*FragmentTransferComplete, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var ftc fragmentTransferCompleteXML
	if err := xml.Unmarshal(env.Body.Content, &ftc); err != nil {
		return nil, fmt.Errorf("failed to parse FragmentTransferComplete: %w", err)
	}

	ft := &FragmentTransferComplete{
		Header: SoapHeader{
			ID: env.Header.ID,
		},
		CommandKey:      ftc.CommandKey,
		TargetFileName:  ftc.TargetFileName,
		FileType:        ftc.FileType,
	}
	if ftc.FaultStruct != nil {
		ft.FaultCode = ftc.FaultStruct.FaultCode
		ft.FaultString = ftc.FaultStruct.FaultString
	}
	return ft, nil
}

// AutonomousFragmentTransferComplete (CPE -> ACS, vendor extension) signals a
// vendor-initiated fragment transfer finished.
//
// Mirrors Java PositiveMessageProcessor.AUTONOMOUS_FRAGMENT_TRANSFER_COMPLETE.
type AutonomousFragmentTransferComplete struct {
	Header         SoapHeader
	CommandKey     string
	FaultCode      int
	FaultString    string
	TargetFileName string
	FileType       string
}

type autonomousFragmentTransferCompleteXML struct {
	XMLName        xml.Name `xml:"AutonomousFragmentTransferComplete"`
	CommandKey     string   `xml:"CommandKey"`
	FaultStruct    *faultStructXML `xml:"FaultStruct"`
	TargetFileName string   `xml:"TargetFileName"`
	FileType       string   `xml:"FileType"`
}

// ParseAutonomousFragmentTransferComplete parses an
// AutonomousFragmentTransferComplete message.
func ParseAutonomousFragmentTransferComplete(xmlStr string) (*AutonomousFragmentTransferComplete, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var aftc autonomousFragmentTransferCompleteXML
	if err := xml.Unmarshal(env.Body.Content, &aftc); err != nil {
		return nil, fmt.Errorf("failed to parse AutonomousFragmentTransferComplete: %w", err)
	}

	a := &AutonomousFragmentTransferComplete{
		Header: SoapHeader{
			ID: env.Header.ID,
		},
		CommandKey:     aftc.CommandKey,
		TargetFileName: aftc.TargetFileName,
		FileType:       aftc.FileType,
	}
	if aftc.FaultStruct != nil {
		a.FaultCode = aftc.FaultStruct.FaultCode
		a.FaultString = aftc.FaultStruct.FaultString
	}
	return a, nil
}
