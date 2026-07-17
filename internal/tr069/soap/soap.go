package soap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CWMP data structures

type DeviceIdStruct struct {
	Manufacturer string
	OUI          string
	ProductClass string
	SerialNumber string
}

type ParameterValueStruct struct {
	Name  string
	Value string
	Type  string // xsi:type: string, int, unsignedInt, boolean, dateTime, base64, etc.
}

type EventStruct struct {
	Code       string // "0 BOOTSTRAP", "1 BOOT", "2 PERIODIC", "4 VALUE CHANGE", "M Reboot", etc.
	CommandKey string
}

type FaultStruct struct {
	FaultCode   int
	FaultString string
}

type SoapHeader struct {
	ID             string
	HoldRequests   bool
	NoMoreRequests bool
}

// GenericResponse represents any generic CWMP response
type GenericResponse struct {
	Header      SoapHeader
	Status      int // 0 = success
	FaultCode   int
	FaultString string
}

// MessageType enum
type MessageType int

const (
	MsgUnknown MessageType = iota
	MsgInform
	MsgInformResponse
	MsgTransferComplete
	MsgAutonomousTransferComplete
	MsgGetParameterValuesResponse
	MsgSetParameterValuesResponse
	MsgGetParameterNamesResponse
	MsgDownloadResponse
	MsgRebootResponse
	MsgUploadResponse
	MsgFactoryResetResponse
	MsgAddObjectResponse
	MsgDeleteObjectResponse
	MsgFault
	MsgGetRPCMethods
	MsgGetRPCMethodsResponse
	MsgCaptureResponse
	MsgFragmentTransferComplete
	MsgHttpRequestProxyResponse
	MsgCancelFutureUpgradeResponse
	MsgSetParameterAttributesResponse
	MsgGetParameterAttributesResponse
	MsgUpdateCBSDStatusResponse
)

// Standard TR-069 CWMP fault codes
const (
	FaultMethodNotSupported    = 9000
	FaultRequestDenied         = 9001
	FaultInternalError         = 9002
	FaultInvalidArguments      = 9003
	FaultResourcesExceeded     = 9004
	FaultRetryRequest          = 9005
	FaultTransferCompleteRetry = 9006
	FaultAuthenticationFailure = 9007
	FaultUnsupportedProtocol   = 9008
)

// Internal XML parsing structures

type soapEnvelope struct {
	XMLName xml.Name    `xml:"Envelope"`
	Header  soapHdrXML  `xml:"Header"`
	Body    soapBodyXML `xml:"Body"`
}

type soapHdrXML struct {
	ID string `xml:"ID"`
}

type soapBodyXML struct {
	Content []byte `xml:",innerxml"`
}

type informXML struct {
	XMLName       xml.Name      `xml:"Inform"`
	DeviceId      deviceIdXML   `xml:"DeviceId"`
	Event         eventListXML  `xml:"Event"`
	MaxEnvelopes  int           `xml:"MaxEnvelopes"`
	CurrentTime   string        `xml:"CurrentTime"`
	RetryCount    int           `xml:"RetryCount"`
	ParameterList parameterLXML `xml:"ParameterList"`
}

type deviceIdXML struct {
	Manufacturer string `xml:"Manufacturer"`
	OUI          string `xml:"OUI"`
	ProductClass string `xml:"ProductClass"`
	SerialNumber string `xml:"SerialNumber"`
}

type eventListXML struct {
	Items []eventStructXML `xml:"EventStruct"`
}

type eventStructXML struct {
	EventCode  string `xml:"EventCode"`
	CommandKey string `xml:"CommandKey"`
}

type parameterLXML struct {
	Items []parameterValueXML `xml:"ParameterValueStruct"`
}

type parameterValueXML struct {
	Name  string   `xml:"Name"`
	Value valueXML `xml:"Value"`
}

type valueXML struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type transferCompleteXML struct {
	XMLName      xml.Name `xml:"TransferComplete"`
	CommandKey   string   `xml:"CommandKey"`
	FaultCode    int      `xml:"FaultCode"`
	FaultString  string   `xml:"FaultString"`
	StartTime    string   `xml:"StartTime"`
	CompleteTime string   `xml:"CompleteTime"`
}

type faultDetailXML struct {
	XMLName     xml.Name `xml:"Fault"`
	FaultCode   int      `xml:"FaultCode"`
	FaultString string   `xml:"FaultString"`
}

// DetectMessageType extracts the CWMP method name from SOAP Body
func DetectMessageType(xmlStr string) MessageType {
	decoder := xml.NewDecoder(strings.NewReader(xmlStr))

	for {
		token, err := decoder.Token()
		if err != nil {
			return MsgUnknown
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "Body" {
				for {
					token, err := decoder.Token()
					if err != nil {
						return MsgUnknown
					}

					switch t := token.(type) {
					case xml.StartElement:
						return methodNameToType(t.Name.Local)
					case xml.EndElement:
						return MsgUnknown
					}
				}
			}
		}
	}
}

func methodNameToType(name string) MessageType {
	switch name {
	case "Inform":
		return MsgInform
	case "InformResponse":
		return MsgInformResponse
	case "TransferComplete":
		return MsgTransferComplete
	case "AutonomousTransferComplete":
		return MsgAutonomousTransferComplete
	case "GetParameterValuesResponse":
		return MsgGetParameterValuesResponse
	case "SetParameterValuesResponse":
		return MsgSetParameterValuesResponse
	case "GetParameterNamesResponse":
		return MsgGetParameterNamesResponse
	case "DownloadResponse":
		return MsgDownloadResponse
	case "RebootResponse":
		return MsgRebootResponse
	case "UploadResponse":
		return MsgUploadResponse
	case "FactoryResetResponse":
		return MsgFactoryResetResponse
	case "AddObjectResponse":
		return MsgAddObjectResponse
	case "DeleteObjectResponse":
		return MsgDeleteObjectResponse
	case "Fault":
		return MsgFault
	case "GetRPCMethods":
		return MsgGetRPCMethods
	case "GetRPCMethodsResponse":
		return MsgGetRPCMethodsResponse
	case "CaptureResponse":
		return MsgCaptureResponse
	case "FragmentTransferComplete":
		return MsgFragmentTransferComplete
	case "HttpRequestProxyResponse":
		return MsgHttpRequestProxyResponse
	case "CancelFutureUpgradeResponse":
		return MsgCancelFutureUpgradeResponse
	case "SetParameterAttributesResponse":
		return MsgSetParameterAttributesResponse
	case "GetParameterAttributesResponse":
		return MsgGetParameterAttributesResponse
	case "UpdateCBSDStatusResponse":
		return MsgUpdateCBSDStatusResponse
	default:
		return MsgUnknown
	}
}

// extractHeaderID extracts the cwmp:ID from a SOAP envelope
func extractHeaderID(xmlStr string) string {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return ""
	}
	return env.Header.ID
}

// ParseGenericResponse parses any generic response
func ParseGenericResponse(xmlStr string) (*GenericResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	result := &GenericResponse{
		Header: SoapHeader{
			ID: env.Header.ID,
		},
	}

	bodyStr := string(env.Body.Content)
	if strings.Contains(bodyStr, "Fault") {
		var fd faultDetailXML
		if err := xml.Unmarshal(env.Body.Content, &fd); err == nil && fd.FaultCode != 0 {
			result.FaultCode = fd.FaultCode
			result.FaultString = fd.FaultString
			result.Status = fd.FaultCode
			return result, nil
		}
	}

	return result, nil
}

// --- Build functions ---

const (
	soapHeader1 = `<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap:Envelope` +
		` xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"` +
		` xmlns:soap-enc="http://schemas.xmlsoap.org/soap/encoding/"` +
		` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"` +
		` xmlns:xsd="http://www.w3.org/2001/XMLSchema"` +
		` xmlns:cwmp="urn:dslforum-org:cwmp-1-0">` +
		`<soap:Header><cwmp:ID>`
	soapHeaderEnd = `</cwmp:ID></soap:Header><soap:Body><cwmp:`
	soapFooter    = `</soap:Body></soap:Envelope>`
)

func writeSoapOpen(b *strings.Builder, headerId string) {
	b.WriteString(soapHeader1)
	b.WriteString(EscapeXML(headerId))
	b.WriteString(soapHeaderEnd)
}

func writeSoapClose(b *strings.Builder, method string) {
	b.WriteString(`</cwmp:`)
	b.WriteString(method)
	b.WriteString(`>`)
	b.WriteString(soapFooter)
}

// BuildFaultResponse builds Fault response SOAP XML
func BuildFaultResponse(headerId string, faultCode int, faultString string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<soap:Envelope`)
	b.WriteString(` xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"`)
	b.WriteString(` xmlns:soap-enc="http://schemas.xmlsoap.org/soap/encoding/"`)
	b.WriteString(` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`)
	b.WriteString(` xmlns:xsd="http://www.w3.org/2001/XMLSchema"`)
	b.WriteString(` xmlns:cwmp="urn:dslforum-org:cwmp-1-0">`)
	b.WriteString(`<soap:Header><cwmp:ID>`)
	b.WriteString(EscapeXML(headerId))
	b.WriteString(`</cwmp:ID></soap:Header>`)
	b.WriteString(`<soap:Body>`)
	b.WriteString(`<soap:Fault>`)
	b.WriteString(`<faultcode>Client</faultcode>`)
	b.WriteString(`<faultstring>`)
	b.WriteString(EscapeXML(faultString))
	b.WriteString(`</faultstring>`)
	b.WriteString(`<detail>`)
	b.WriteString(`<cwmp:Fault>`)
	b.WriteString(`<FaultCode>`)
	b.WriteString(strconv.Itoa(faultCode))
	b.WriteString(`</FaultCode>`)
	b.WriteString(`<FaultString>`)
	b.WriteString(EscapeXML(faultString))
	b.WriteString(`</FaultString>`)
	b.WriteString(`</cwmp:Fault>`)
	b.WriteString(`</detail>`)
	b.WriteString(`</soap:Fault>`)
	b.WriteString(`</soap:Body>`)
	b.WriteString(`</soap:Envelope>`)
	return b.String()
}

// --- Utility functions ---

// GenerateHeaderID generates a unique cwmp:ID based on timestamp
func GenerateHeaderID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// EscapeXML escapes special XML characters
func EscapeXML(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// BuildParameterValueXML builds a single ParameterValueStruct XML fragment
func BuildParameterValueXML(pvs ParameterValueStruct) string {
	var b strings.Builder
	b.WriteString(`<ParameterValueStruct><Name>`)
	b.WriteString(EscapeXML(pvs.Name))
	b.WriteString(`</Name><Value xsi:type="`)

	valueType := pvs.Type
	if valueType == "" {
		valueType = "xsd:string"
	}
	// Ensure the type has the proper namespace prefix
	if !strings.Contains(valueType, ":") {
		valueType = "xsd:" + valueType
	}

	b.WriteString(EscapeXML(valueType))
	b.WriteString(`">`)
	b.WriteString(EscapeXML(pvs.Value))
	b.WriteString(`</Value></ParameterValueStruct>`)
	return b.String()
}
