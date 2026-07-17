package soap

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// BuildSetParameterAttributes builds SetParameterAttributes request SOAP XML
func BuildSetParameterAttributes(headerId string, spa *SetParameterAttributes) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`SetParameterAttributes><ParameterList soap-enc:arrayType="cwmp:SetParameterAttributesStruct[`)
	b.WriteString(strconv.Itoa(len(spa.ParameterList)))
	b.WriteString(`]">`)
	for _, p := range spa.ParameterList {
		b.WriteString(`<SetParameterAttributesStruct>`)
		b.WriteString(`<Name>`)
		b.WriteString(EscapeXML(p.Name))
		b.WriteString(`</Name>`)
		b.WriteString(`<Notification>`)
		b.WriteString(strconv.Itoa(p.Notification))
		b.WriteString(`</Notification>`)
		b.WriteString(`<NotificationChange>`)
		b.WriteString(strconv.FormatBool(p.NotificationChange))
		b.WriteString(`</NotificationChange>`)
		b.WriteString(`<AccessListChange>`)
		b.WriteString(strconv.FormatBool(p.AccessListChange))
		b.WriteString(`</AccessListChange>`)
		b.WriteString(`<AccessList soap-enc:arrayType="cwmp:string[`)
		b.WriteString(strconv.Itoa(len(p.AccessList)))
		b.WriteString(`]">`)
		for _, a := range p.AccessList {
			b.WriteString(`<string>`)
			b.WriteString(EscapeXML(a))
			b.WriteString(`</string>`)
		}
		b.WriteString(`</AccessList>`)
		b.WriteString(`</SetParameterAttributesStruct>`)
	}
	b.WriteString(`</ParameterList>`)
	writeSoapClose(&b, "SetParameterAttributes")
	return b.String()
}

// BuildGetParameterAttributes builds GetParameterAttributes request SOAP XML
func BuildGetParameterAttributes(headerId string, names []string) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`GetParameterAttributes><ParameterNames soap-enc:arrayType="xsd:string[`)
	b.WriteString(strconv.Itoa(len(names)))
	b.WriteString(`]">`)
	for _, name := range names {
		b.WriteString(`<string>`)
		b.WriteString(EscapeXML(name))
		b.WriteString(`</string>`)
	}
	b.WriteString(`</ParameterNames>`)
	writeSoapClose(&b, "GetParameterAttributes")
	return b.String()
}

// BuildUpdateCBSDStatus builds UpdateCBSDStatus request SOAP XML (vendor extension)
func BuildUpdateCBSDStatus(headerId string, ucs *UpdateCBSDStatus) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`UpdateCBSDStatus><CBSDInfos soap-enc:arrayType="cwmp:CBSDInfo[`)
	b.WriteString(strconv.Itoa(len(ucs.CBSDInfos)))
	b.WriteString(`]">`)
	for _, info := range ucs.CBSDInfos {
		b.WriteString(`<CBSDInfo>`)
		b.WriteString(`<State>`)
		b.WriteString(EscapeXML(info.State))
		b.WriteString(`</State>`)
		if info.CBSDSerialNumber != "" {
			b.WriteString(`<CBSDSerialNumber>`)
			b.WriteString(EscapeXML(info.CBSDSerialNumber))
			b.WriteString(`</CBSDSerialNumber>`)
		}
		if info.TxPower != nil {
			b.WriteString(`<TxPower>`)
			b.WriteString(strconv.Itoa(*info.TxPower))
			b.WriteString(`</TxPower>`)
		}
		if info.LowFrequency != nil {
			b.WriteString(`<LowFrequency>`)
			b.WriteString(strconv.FormatInt(*info.LowFrequency, 10))
			b.WriteString(`</LowFrequency>`)
		}
		if info.HighFrequency != nil {
			b.WriteString(`<HighFrequency>`)
			b.WriteString(strconv.FormatInt(*info.HighFrequency, 10))
			b.WriteString(`</HighFrequency>`)
		}
		if info.TransmitExpireTime != "" {
			b.WriteString(`<TransmitExpireTime>`)
			b.WriteString(EscapeXML(info.TransmitExpireTime))
			b.WriteString(`</TransmitExpireTime>`)
		}
		b.WriteString(`</CBSDInfo>`)
	}
	b.WriteString(`</CBSDInfos>`)
	writeSoapClose(&b, "UpdateCBSDStatus")
	return b.String()
}

// BuildShellDownload builds ShellDownload request SOAP XML (vendor extension with UploadURL)
func BuildShellDownload(headerId string, sd *ShellDownload) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`ShellDownload>`)
	b.WriteString(`<CommandKey>`)
	b.WriteString(EscapeXML(sd.CommandKey))
	b.WriteString(`</CommandKey>`)
	b.WriteString(`<FileType>`)
	b.WriteString(EscapeXML(sd.FileType))
	b.WriteString(`</FileType>`)
	b.WriteString(`<URL>`)
	b.WriteString(EscapeXML(sd.URL))
	b.WriteString(`</URL>`)
	b.WriteString(`<Username>`)
	b.WriteString(EscapeXML(sd.Username))
	b.WriteString(`</Username>`)
	b.WriteString(`<Password>`)
	b.WriteString(EscapeXML(sd.Password))
	b.WriteString(`</Password>`)
	b.WriteString(`<FileSize>`)
	b.WriteString(strconv.Itoa(sd.FileSize))
	b.WriteString(`</FileSize>`)
	b.WriteString(`<TargetFileName>`)
	b.WriteString(EscapeXML(sd.TargetFileName))
	b.WriteString(`</TargetFileName>`)
	b.WriteString(`<DelaySeconds>`)
	b.WriteString(strconv.Itoa(sd.DelaySeconds))
	b.WriteString(`</DelaySeconds>`)
	b.WriteString(`<SuccessURL>`)
	b.WriteString(EscapeXML(sd.SuccessURL))
	b.WriteString(`</SuccessURL>`)
	b.WriteString(`<FailureURL>`)
	b.WriteString(EscapeXML(sd.FailureURL))
	b.WriteString(`</FailureURL>`)
	b.WriteString(`<UploadURL>`)
	b.WriteString(EscapeXML(sd.UploadURL))
	b.WriteString(`</UploadURL>`)
	writeSoapClose(&b, "ShellDownload")
	return b.String()
}

// CBSDFaultInfo mirrors Java CBSDFaultInfo — a single fault entry in
// UpdateCBSDStatusResponse.
type CBSDFaultInfo struct {
	CBSDSerialNumber string `xml:"CBSDSerialNumber"`
	FaultCode        int    `xml:"FaultCode"`
	Bandwidth        int    `xml:"Bandwidth"`
	CellId           string `xml:"cellId"`
}

type updateCbsdStatusRespXML struct {
	XMLName    xml.Name         `xml:"UpdateCBSDStatusResponse"`
	FaultInfos []CBSDFaultInfo  `xml:"FaultInfos>CBSDFaultInfo"`
}

// UpdateCBSDStatusResponse mirrors Java UpdateCBSDStatusResponse.
type UpdateCBSDStatusResponse struct {
	Header         SoapHeader
	CBSDFaultInfos []CBSDFaultInfo
}

// ParseUpdateCBSDStatusResponse parses an UpdateCBSDStatusResponse SOAP
// envelope (CPE -> ACS). It mirrors Java UpdateCBSDStatusHandler.parseXml.
func ParseUpdateCBSDStatusResponse(xmlStr string) (*UpdateCBSDStatusResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	var resp updateCbsdStatusRespXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse UpdateCBSDStatusResponse: %w", err)
	}
	return &UpdateCBSDStatusResponse{
		Header:         SoapHeader{ID: env.Header.ID},
		CBSDFaultInfos: resp.FaultInfos,
	}, nil
}

// BuildHttpRequestProxy builds HttpRequestProxy request SOAP XML (vendor extension)
func BuildHttpRequestProxy(headerId string, proxy *HttpRequestProxy) string {
	var b strings.Builder
	writeSoapOpen(&b, headerId)
	b.WriteString(`HttpRequestProxy><HttpRequests soap-enc:arrayType="xsd:HttpRequest[`)
	b.WriteString(strconv.Itoa(len(proxy.Requests)))
	b.WriteString(`]">`)
	for _, req := range proxy.Requests {
		b.WriteString(`<HttpRequest>`)
		b.WriteString(`<Url>`)
		b.WriteString(EscapeXML(req.URL))
		b.WriteString(`</Url>`)
		b.WriteString(`<HttpMethod>`)
		b.WriteString(EscapeXML(req.HttpMethod))
		b.WriteString(`</HttpMethod>`)
		if req.Body != "" {
			b.WriteString(`<Body>`)
			b.WriteString(EscapeXML(req.Body))
			b.WriteString(`</Body>`)
		}
		b.WriteString(`<RequestId>`)
		b.WriteString(EscapeXML(req.RequestId))
		b.WriteString(`</RequestId>`)
		b.WriteString(`</HttpRequest>`)
	}
	b.WriteString(`</HttpRequests>`)
	writeSoapClose(&b, "HttpRequestProxy")
	return b.String()
}
