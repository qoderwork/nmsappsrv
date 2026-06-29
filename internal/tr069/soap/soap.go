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

// Download request (ACS -> CPE)
type Download struct {
	CommandKey     string
	FileType       string
	URL            string
	Username       string
	Password       string
	FileSize       int
	TargetFileName string
	DelaySeconds   int
	SuccessURL     string
	FailureURL     string
}

// GetParameterValues request (ACS -> CPE)
type GetParameterValues struct {
	ParameterNames []string
}

// SetParameterValues request (ACS -> CPE)
type SetParameterValues struct {
	ParameterList []ParameterValueStruct
	ParameterKey  string
}

// GetParameterNames request (ACS -> CPE)
type GetParameterNames struct {
	ParameterPath string
	NextLevel     bool
}

// Reboot request (ACS -> CPE)
type Reboot struct {
	CommandKey string
}

// Upload request (ACS -> CPE)
type Upload struct {
	CommandKey string
	FileType   string
	URL        string
	Username   string
	Password   string
}

// FactoryReset (ACS -> CPE)
type FactoryReset struct{}

// AddObject (ACS -> CPE)
type AddObject struct {
	ObjectName   string
	ParameterKey string
}

// DeleteObject (ACS -> CPE)
type DeleteObject struct {
	ObjectName   string
	ParameterKey string
}

// TransferComplete (CPE -> ACS)
type TransferComplete struct {
	Header       SoapHeader
	CommandKey   string
	FaultCode    int
	FaultString  string
	StartTime    string
	CompleteTime string
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
	MsgGetRPCMethodsResponse
	MsgCaptureResponse
	MsgFragmentTransferComplete
	MsgHttpRequestProxyResponse
	MsgCancelFutureUpgradeResponse
	MsgSetParameterAttributesResponse
	MsgGetParameterAttributesResponse
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
				// Find the first child element inside Body
				for {
					token, err := decoder.Token()
					if err != nil {
						return MsgUnknown
					}

					switch t := token.(type) {
					case xml.StartElement:
						return methodNameToType(t.Name.Local)
					case xml.EndElement:
						// Empty body
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

	// Try to parse as a SOAP Fault first
	bodyStr := string(env.Body.Content)
	if strings.Contains(bodyStr, "Fault") {
		var fd faultDetailXML
		// Try to find cwmp:Fault in detail
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

// BuildInformResponse builds InformResponse SOAP XML
func BuildInformResponse(headerId string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`InformResponse><MaxEnvelopes>1</MaxEnvelopes>`)
	writeSoapClose(&b, "InformResponse")
	return b.String()
}

// BuildGetParameterValues builds GPV request SOAP XML
func BuildGetParameterValues(headerId string, names []string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`GetParameterValues><ParameterNames soap-enc:arrayType="xsd:string[`)
	b.WriteString(strconv.Itoa(len(names)))
	b.WriteString(`]">`)
	for _, name := range names {
		b.WriteString(`<string>`)
		b.WriteString(EscapeXML(name))
		b.WriteString(`</string>`)
	}
	b.WriteString(`</ParameterNames>`)
	writeSoapClose(&b, "GetParameterValues")
	return b.String()
}

// BuildSetParameterValues builds SPV request SOAP XML
func BuildSetParameterValues(headerId string, params []ParameterValueStruct, paramKey string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`SetParameterValues><ParameterList soap-enc:arrayType="cwmp:ParameterValueStruct[`)
	b.WriteString(strconv.Itoa(len(params)))
	b.WriteString(`]">`)
	for _, param := range params {
		b.WriteString(BuildParameterValueXML(param))
	}
	b.WriteString(`</ParameterList>`)
	if paramKey != "" {
		b.WriteString(`<ParameterKey>`)
		b.WriteString(EscapeXML(paramKey))
		b.WriteString(`</ParameterKey>`)
	}
	writeSoapClose(&b, "SetParameterValues")
	return b.String()
}

// BuildGetParameterNames builds GPN request SOAP XML
func BuildGetParameterNames(headerId string, path string, nextLevel bool) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`GetParameterNames><ParameterPath>`)
	b.WriteString(EscapeXML(path))
	b.WriteString(`</ParameterPath><NextLevel>`)
	if nextLevel {
		b.WriteString(`1`)
	} else {
		b.WriteString(`0`)
	}
	b.WriteString(`</NextLevel>`)
	writeSoapClose(&b, "GetParameterNames")
	return b.String()
}

// BuildDownload builds Download request SOAP XML
func BuildDownload(headerId string, dl *Download) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`Download>`)
	b.WriteString(`<CommandKey>`)
	b.WriteString(EscapeXML(dl.CommandKey))
	b.WriteString(`</CommandKey>`)
	b.WriteString(`<FileType>`)
	b.WriteString(EscapeXML(dl.FileType))
	b.WriteString(`</FileType>`)
	b.WriteString(`<URL>`)
	b.WriteString(EscapeXML(dl.URL))
	b.WriteString(`</URL>`)
	b.WriteString(`<Username>`)
	b.WriteString(EscapeXML(dl.Username))
	b.WriteString(`</Username>`)
	b.WriteString(`<Password>`)
	b.WriteString(EscapeXML(dl.Password))
	b.WriteString(`</Password>`)
	b.WriteString(`<FileSize>`)
	b.WriteString(strconv.Itoa(dl.FileSize))
	b.WriteString(`</FileSize>`)
	b.WriteString(`<TargetFileName>`)
	b.WriteString(EscapeXML(dl.TargetFileName))
	b.WriteString(`</TargetFileName>`)
	b.WriteString(`<DelaySeconds>`)
	b.WriteString(strconv.Itoa(dl.DelaySeconds))
	b.WriteString(`</DelaySeconds>`)
	b.WriteString(`<SuccessURL>`)
	b.WriteString(EscapeXML(dl.SuccessURL))
	b.WriteString(`</SuccessURL>`)
	b.WriteString(`<FailureURL>`)
	b.WriteString(EscapeXML(dl.FailureURL))
	b.WriteString(`</FailureURL>`)
	writeSoapClose(&b, "Download")
	return b.String()
}

// BuildReboot builds Reboot request SOAP XML
func BuildReboot(headerId string, commandKey string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`Reboot><CommandKey>`)
	b.WriteString(EscapeXML(commandKey))
	b.WriteString(`</CommandKey>`)
	writeSoapClose(&b, "Reboot")
	return b.String()
}

// BuildUpload builds Upload request SOAP XML
func BuildUpload(headerId string, upload *Upload) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`Upload>`)
	b.WriteString(`<CommandKey>`)
	b.WriteString(EscapeXML(upload.CommandKey))
	b.WriteString(`</CommandKey>`)
	b.WriteString(`<FileType>`)
	b.WriteString(EscapeXML(upload.FileType))
	b.WriteString(`</FileType>`)
	b.WriteString(`<URL>`)
	b.WriteString(EscapeXML(upload.URL))
	b.WriteString(`</URL>`)
	b.WriteString(`<Username>`)
	b.WriteString(EscapeXML(upload.Username))
	b.WriteString(`</Username>`)
	b.WriteString(`<Password>`)
	b.WriteString(EscapeXML(upload.Password))
	b.WriteString(`</Password>`)
	writeSoapClose(&b, "Upload")
	return b.String()
}

// BuildFactoryReset builds FactoryReset request SOAP XML
func BuildFactoryReset(headerId string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`FactoryReset/>`)
	// FactoryReset is self-closing, so we need a different close
	b.WriteString(soapFooter)
	return b.String()
}

// BuildAddObject builds AddObject request SOAP XML
func BuildAddObject(headerId string, objectName string, paramKey string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`AddObject><ObjectName>`)
	b.WriteString(EscapeXML(objectName))
	b.WriteString(`</ObjectName>`)
	if paramKey != "" {
		b.WriteString(`<ParameterKey>`)
		b.WriteString(EscapeXML(paramKey))
		b.WriteString(`</ParameterKey>`)
	}
	writeSoapClose(&b, "AddObject")
	return b.String()
}

// BuildDeleteObject builds DeleteObject request SOAP XML
func BuildDeleteObject(headerId string, objectName string, paramKey string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`DeleteObject><ObjectName>`)
	b.WriteString(EscapeXML(objectName))
	b.WriteString(`</ObjectName>`)
	if paramKey != "" {
		b.WriteString(`<ParameterKey>`)
		b.WriteString(EscapeXML(paramKey))
		b.WriteString(`</ParameterKey>`)
	}
	writeSoapClose(&b, "DeleteObject")
	return b.String()
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
