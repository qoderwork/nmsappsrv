package soap

import (
	"strconv"
	"strings"
)

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

// Capture (ACS -> CPE) vendor extension for packet capture
type Capture struct {
	CommandKey      string
	CaptureType     string
	CaptureOptional string
	FAPI            string
	Size            int
	CaptureSwitch   string
	URL             string
	Username        string
	Password        string
	TransmitIP      string
}

// UpgradeDeviceStruct - single device entry in BatchUpgrade
type UpgradeDeviceStruct struct {
	DeviceRouteList []string
	URL             string
	FileSize        int64
	TargetFileName  string
}

// BatchUpgrade (ACS -> CPE) - upgrade multiple devices in one RPC
type BatchUpgrade struct {
	CommandKey        string
	UpgradeDeviceList []UpgradeDeviceStruct
}

// HttpRequest - single HTTP request in HttpRequestProxy
type HttpRequest struct {
	URL        string
	HttpMethod string
	Body       string
	RequestId  string
}

// HttpRequestProxy (ACS -> CPE) - proxy HTTP requests through CPE
type HttpRequestProxy struct {
	Requests []HttpRequest
}

// SetParameterAttributesStruct - single parameter attribute entry
type SetParameterAttributesStruct struct {
	Name               string
	Notification       int
	NotificationChange bool
	AccessListChange   bool
	AccessList         []string
}

// SetParameterAttributes (ACS -> CPE)
type SetParameterAttributes struct {
	ParameterList []SetParameterAttributesStruct
}

// CBSDInfo - single CBSD entry in UpdateCBSDStatus
type CBSDInfo struct {
	State              string
	CBSDSerialNumber   string
	TxPower            *int
	LowFrequency       *int64
	HighFrequency      *int64
	TransmitExpireTime string
}

// UpdateCBSDStatus (ACS -> CPE) - update CBSD status info
type UpdateCBSDStatus struct {
	CBSDInfos []CBSDInfo
}

// ShellDownload (ACS -> CPE) - like Download but with extra UploadURL field
type ShellDownload struct {
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
	UploadURL      string
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

// BuildGetRPCMethods builds GetRPCMethods request SOAP XML
func BuildGetRPCMethods(headerId string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`GetRPCMethods/>`)
	b.WriteString(soapFooter)
	return b.String()
}

// supportedRPCMethods lists the CWMP methods this ACS supports.
var supportedRPCMethods = []string{
	"cwmp:GetParameterValues",
	"cwmp:SetParameterValues",
	"cwmp:GetParameterNames",
	"cwmp:AddObject",
	"cwmp:DeleteObject",
	"cwmp:Download",
	"cwmp:Upload",
	"cwmp:Reboot",
	"cwmp:FactoryReset",
	"cwmp:GetRPCMethods",
	"cwmp:SetParameterAttributes",
	"cwmp:GetParameterAttributes",
	"cwmp:SoftReboot",
}

// BuildGetRPCMethodsResponse builds a GetRPCMethodsResponse SOAP XML listing supported methods.
func BuildGetRPCMethodsResponse(headerId string) string {
	return BuildGetRPCMethodsResponseWithNS(headerId, CWMPNamespace1_0)
}

// BuildGetRPCMethodsResponseWithNS builds a GetRPCMethodsResponse SOAP XML with specified CWMP namespace
func BuildGetRPCMethodsResponseWithNS(headerId string, ns string) string {
	var b strings.Builder
	writeSoapOpenWithNS(&b, headerId, ns)
	b.WriteString(`GetRPCMethodsResponse><MethodList soap-enc:arrayType="xsd:string[`)
	b.WriteString(strconv.Itoa(len(supportedRPCMethods)))
	b.WriteString(`]">`)
	for _, method := range supportedRPCMethods {
		b.WriteString(`<string>`)
		b.WriteString(EscapeXML(method))
		b.WriteString(`</string>`)
	}
	b.WriteString(`</MethodList>`)
	writeSoapClose(&b, "GetRPCMethodsResponse")
	return b.String()
}

// BuildCapture builds Capture request SOAP XML (vendor extension)
func BuildCapture(headerId string, cap *Capture) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`Capture>`)
	if cap.CommandKey != "" {
		b.WriteString(`<CommandKey>`)
		b.WriteString(EscapeXML(cap.CommandKey))
		b.WriteString(`</CommandKey>`)
	}
	if cap.CaptureType != "" {
		b.WriteString(`<CaptureType>`)
		b.WriteString(EscapeXML(cap.CaptureType))
		b.WriteString(`</CaptureType>`)
	}
	if cap.CaptureOptional != "" {
		b.WriteString(`<CaptureOptional>`)
		b.WriteString(EscapeXML(cap.CaptureOptional))
		b.WriteString(`</CaptureOptional>`)
	}
	if cap.FAPI != "" {
		b.WriteString(`<FAPI>`)
		b.WriteString(EscapeXML(cap.FAPI))
		b.WriteString(`</FAPI>`)
	}
	if cap.Size > 0 {
		b.WriteString(`<Size>`)
		b.WriteString(strconv.Itoa(cap.Size))
		b.WriteString(`</Size>`)
	}
	if cap.CaptureSwitch != "" {
		b.WriteString(`<CaptureSwitch>`)
		b.WriteString(EscapeXML(cap.CaptureSwitch))
		b.WriteString(`</CaptureSwitch>`)
	}
	if cap.URL != "" {
		b.WriteString(`<URL>`)
		b.WriteString(EscapeXML(cap.URL))
		b.WriteString(`</URL>`)
	}
	if cap.Username != "" {
		b.WriteString(`<Username>`)
		b.WriteString(EscapeXML(cap.Username))
		b.WriteString(`</Username>`)
	}
	if cap.Password != "" {
		b.WriteString(`<Password>`)
		b.WriteString(EscapeXML(cap.Password))
		b.WriteString(`</Password>`)
	}
	if cap.TransmitIP != "" {
		b.WriteString(`<TransmitIp>`)
		b.WriteString(EscapeXML(cap.TransmitIP))
		b.WriteString(`</TransmitIp>`)
	}
	writeSoapClose(&b, "Capture")
	return b.String()
}

// BuildSoftReboot builds SoftReboot request SOAP XML
func BuildSoftReboot(headerId string, commandKey string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`SoftReboot><CommandKey>`)
	b.WriteString(EscapeXML(commandKey))
	b.WriteString(`</CommandKey>`)
	writeSoapClose(&b, "SoftReboot")
	return b.String()
}

// BuildReportTransmissionProgressResponse builds the empty
// ReportTransmissionProgressResponse SOAP envelope that the ACS returns
// to a CPE after processing a ReportTransmissionProgress notification.
//
// Mirrors Java NgLogProcessHandler.build().
func BuildReportTransmissionProgressResponse(headerId string) string {
	return BuildReportTransmissionProgressResponseWithNS(headerId, CWMPNamespace1_0)
}

func BuildReportTransmissionProgressResponseWithNS(headerId string, ns string) string {
	var b strings.Builder
	b.WriteString(soapHeaderWithNS(ns))
	b.WriteString(EscapeXML(headerId))
	b.WriteString(`</cwmp:ID></soap:Header>`)
	b.WriteString(`<soap:Body>`)
	b.WriteString(`<cwmp:ReportTransmissionProgressResponse/>`)
	b.WriteString(`</soap:Body>`)
	b.WriteString(`</soap:Envelope>`)
	return b.String()
}

// BuildBatchUpgrade builds BatchUpgrade request SOAP XML (vendor extension)
func BuildBatchUpgrade(headerId string, batch *BatchUpgrade) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`BatchUpgrade><CommandKey>`)
	b.WriteString(EscapeXML(batch.CommandKey))
	b.WriteString(`</CommandKey>`)
	b.WriteString(`<UpgradeDeviceList soap-enc:arrayType="cwmp:UpgradeDeviceStruct[`)
	b.WriteString(strconv.Itoa(len(batch.UpgradeDeviceList)))
	b.WriteString(`]">`)
	for _, dev := range batch.UpgradeDeviceList {
		b.WriteString(`<UpgradeDeviceStruct>`)
		b.WriteString(`<DeviceRouteList soap-enc:arrayType="xsd:string[`)
		b.WriteString(strconv.Itoa(len(dev.DeviceRouteList)))
		b.WriteString(`]">`)
		for _, route := range dev.DeviceRouteList {
			b.WriteString(`<string>`)
			b.WriteString(EscapeXML(route))
			b.WriteString(`</string>`)
		}
		b.WriteString(`</DeviceRouteList>`)
		b.WriteString(`<URL>`)
		b.WriteString(EscapeXML(dev.URL))
		b.WriteString(`</URL>`)
		b.WriteString(`<FileSize>`)
		b.WriteString(strconv.FormatInt(dev.FileSize, 10))
		b.WriteString(`</FileSize>`)
		b.WriteString(`<TargetFileName>`)
		b.WriteString(EscapeXML(dev.TargetFileName))
		b.WriteString(`</TargetFileName>`)
		b.WriteString(`</UpgradeDeviceStruct>`)
	}
	b.WriteString(`</UpgradeDeviceList>`)
	writeSoapClose(&b, "BatchUpgrade")
	return b.String()
}

// BuildCancelFutureUpgrade builds CancelFutureUpgrade request SOAP XML (vendor extension)
func BuildCancelFutureUpgrade(headerId string, commandKey string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`CancelFutureUpgrade><CommandKey>`)
	b.WriteString(EscapeXML(commandKey))
	b.WriteString(`</CommandKey>`)
	writeSoapClose(&b, "CancelFutureUpgrade")
	return b.String()
}

