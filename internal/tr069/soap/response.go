package soap

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// --- Response parse functions ---

// gpvResponseXML is the internal XML structure for GetParameterValuesResponse.
type gpvResponseXML struct {
	XMLName       xml.Name        `xml:"GetParameterValuesResponse"`
	ParameterList gpvParamListXML `xml:"ParameterList"`
}

type gpvParamListXML struct {
	Items []parameterValueXML `xml:"ParameterValueStruct"`
}

// ParseGetParameterValuesResponse parses a GetParameterValuesResponse SOAP message
// and returns the list of parameter name/value/type triples.
func ParseGetParameterValuesResponse(xmlStr string) ([]ParameterValueStruct, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var resp gpvResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GetParameterValuesResponse: %w", err)
	}

	var params []ParameterValueStruct
	for _, p := range resp.ParameterList.Items {
		params = append(params, ParameterValueStruct{
			Name:  p.Name,
			Value: strings.TrimSpace(p.Value.Value),
			Type:  p.Value.Type,
		})
	}
	return params, nil
}

// gpnResponseXML is the internal XML structure for GetParameterNamesResponse.
type gpnResponseXML struct {
	XMLName       xml.Name        `xml:"GetParameterNamesResponse"`
	ParameterList gpnParamListXML `xml:"ParameterList"`
}

type gpnParamListXML struct {
	Items []gpnParamInfoXML `xml:"ParameterInfoStruct"`
}

type gpnParamInfoXML struct {
	Name     string `xml:"Name"`
	Writable bool   `xml:"Writable"`
}

// ParameterInfoStruct represents a single parameter info entry from GetParameterNamesResponse.
type ParameterInfoStruct struct {
	Name     string
	Writable bool
}

// ParseGetParameterNamesResponse parses a GetParameterNamesResponse SOAP message
// and returns the list of parameter name/writable pairs.
func ParseGetParameterNamesResponse(xmlStr string) ([]ParameterInfoStruct, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var resp gpnResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GetParameterNamesResponse: %w", err)
	}

	var params []ParameterInfoStruct
	for _, p := range resp.ParameterList.Items {
		params = append(params, ParameterInfoStruct{
			Name:     p.Name,
			Writable: p.Writable,
		})
	}
	return params, nil
}

// spvResponseXML is the internal XML structure for SetParameterValuesResponse.
type spvResponseXML struct {
	XMLName xml.Name `xml:"SetParameterValuesResponse"`
	Status  int      `xml:"Status"`
}

// ParseSetParameterValuesResponse parses a SetParameterValuesResponse SOAP message
// and returns the status code (0 = success).
func ParseSetParameterValuesResponse(xmlStr string) (int, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return -1, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}

	var resp spvResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return -1, fmt.Errorf("failed to parse SetParameterValuesResponse: %w", err)
	}

	return resp.Status, nil
}

// --- DownloadResponse ---

// DownloadResponse represents the parsed DownloadResponse SOAP message.
type DownloadResponse struct {
	Status      int    `xml:"Status"`
	StartTime   string `xml:"StartTime"`
	CompleteTime string `xml:"CompleteTime"`
}

type downloadResponseXML struct {
	XMLName      xml.Name `xml:"DownloadResponse"`
	Status       int      `xml:"Status"`
	StartTime    string   `xml:"StartTime"`
	CompleteTime string   `xml:"CompleteTime"`
}

// ParseDownloadResponse parses a DownloadResponse SOAP message.
// Status: 0=download not started yet (accepted), 1=download in progress, other=error.
func ParseDownloadResponse(xmlStr string) (*DownloadResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	var resp downloadResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse DownloadResponse: %w", err)
	}
	return &DownloadResponse{
		Status:       resp.Status,
		StartTime:    resp.StartTime,
		CompleteTime: resp.CompleteTime,
	}, nil
}

// --- UploadResponse ---

type uploadResponseXML struct {
	XMLName      xml.Name `xml:"UploadResponse"`
	Status       int      `xml:"Status"`
	StartTime    string   `xml:"StartTime"`
	CompleteTime string   `xml:"CompleteTime"`
}

// UploadResponse represents the parsed UploadResponse SOAP message.
type UploadResponse struct {
	Status      int
	StartTime   string
	CompleteTime string
}

// ParseUploadResponse parses an UploadResponse SOAP message.
func ParseUploadResponse(xmlStr string) (*UploadResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	var resp uploadResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse UploadResponse: %w", err)
	}
	return &UploadResponse{
		Status:       resp.Status,
		StartTime:    resp.StartTime,
		CompleteTime: resp.CompleteTime,
	}, nil
}

// --- BatchUpgradeResponse ---

type batchUpgradeResponseXML struct {
	XMLName  xml.Name `xml:"BatchUpgradeResponse"`
	Status   int      `xml:"Status"`
	FailCase string   `xml:"FailCase"`
}

// BatchUpgradeResponse represents the parsed BatchUpgradeResponse SOAP message.
type BatchUpgradeResponse struct {
	Status   int
	FailCase string
}

// ParseBatchUpgradeResponse parses a BatchUpgradeResponse SOAP message.
func ParseBatchUpgradeResponse(xmlStr string) (*BatchUpgradeResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	var resp batchUpgradeResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse BatchUpgradeResponse: %w", err)
	}
	return &BatchUpgradeResponse{
		Status:   resp.Status,
		FailCase: resp.FailCase,
	}, nil
}

// --- CancelFutureUpgradeResponse ---

type cancelFutureUpgradeResponseXML struct {
	XMLName    xml.Name `xml:"CancelFutureUpgradeResponse"`
	CommandKey string   `xml:"CommandKey"`
}

// CancelFutureUpgradeResponse represents the parsed CancelFutureUpgradeResponse SOAP message.
type CancelFutureUpgradeResponse struct {
	CommandKey string
}

// ParseCancelFutureUpgradeResponse parses a CancelFutureUpgradeResponse SOAP message.
func ParseCancelFutureUpgradeResponse(xmlStr string) (*CancelFutureUpgradeResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	var resp cancelFutureUpgradeResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse CancelFutureUpgradeResponse: %w", err)
	}
	return &CancelFutureUpgradeResponse{
		CommandKey: resp.CommandKey,
	}, nil
}

// --- HttpRequestProxyResponse ---

// HttpRespEntry represents a single HTTP response in HttpRequestProxyResponse.
type HttpRespEntry struct {
	Status       int    `xml:"Status"`
	ResponseCode string `xml:"ResponseCode"`
	Body         string `xml:"Body"`
	RequestId    string `xml:"RequestId"`
}

type httpRequestProxyResponseXML struct {
	XMLName   xml.Name       `xml:"HttpRequestProxyResponse"`
	Responses []HttpRespEntry `xml:"HttpResponses>HttpResponse"`
}

// HttpRequestProxyResponse represents the parsed HttpRequestProxyResponse SOAP message.
type HttpRequestProxyResponse struct {
	Responses []HttpRespEntry
}

// ParseHttpRequestProxyResponse parses an HttpRequestProxyResponse SOAP message.
func ParseHttpRequestProxyResponse(xmlStr string) (*HttpRequestProxyResponse, error) {
	var env soapEnvelope
	if err := xml.Unmarshal([]byte(xmlStr), &env); err != nil {
		return nil, fmt.Errorf("failed to parse SOAP envelope: %w", err)
	}
	var resp httpRequestProxyResponseXML
	if err := xml.Unmarshal(env.Body.Content, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse HttpRequestProxyResponse: %w", err)
	}
	return &HttpRequestProxyResponse{
		Responses: resp.Responses,
	}, nil
}
