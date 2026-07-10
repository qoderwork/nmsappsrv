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
