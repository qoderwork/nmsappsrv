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
