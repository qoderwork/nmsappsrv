package soap

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Inform message (CPE -> ACS)
type Inform struct {
	Header        SoapHeader
	DeviceId      DeviceIdStruct
	EventList     []EventStruct
	MaxEnvelopes  int
	CurrentTime   string
	RetryCount    int
	ParameterList []ParameterValueStruct
}

// ParseInform parses Inform message from CPE
func ParseInform(xmlStr string) (*Inform, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var inf informXML
	if err := xml.Unmarshal(env.Body.Content, &inf); err != nil {
		return nil, fmt.Errorf("failed to parse Inform: %w", err)
	}

	result := &Inform{
		Header: SoapHeader{
			ID: env.Header.ID,
		},
		DeviceId: DeviceIdStruct{
			Manufacturer: inf.DeviceId.Manufacturer,
			OUI:          inf.DeviceId.OUI,
			ProductClass: inf.DeviceId.ProductClass,
			SerialNumber: inf.DeviceId.SerialNumber,
		},
		MaxEnvelopes: inf.MaxEnvelopes,
		CurrentTime:  inf.CurrentTime,
		RetryCount:   inf.RetryCount,
	}

	for _, evt := range inf.Event.Items {
		result.EventList = append(result.EventList, EventStruct{
			Code:       evt.EventCode,
			CommandKey: evt.CommandKey,
		})
	}

	for _, param := range inf.ParameterList.Items {
		result.ParameterList = append(result.ParameterList, ParameterValueStruct{
			Name:  param.Name,
			Value: strings.TrimSpace(param.Value.Value),
			Type:  param.Value.Type,
		})
	}

	return result, nil
}

// BuildInformResponse builds InformResponse SOAP XML
func BuildInformResponse(headerId string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`InformResponse><MaxEnvelopes>1</MaxEnvelopes>`)
	writeSoapClose(&b, "InformResponse")
	return b.String()
}
