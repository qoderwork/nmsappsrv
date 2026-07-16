package corenet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// elementLoopbackIP maps a core-network element type to the loopback
// management address it listens on (port 33030). Mirrors Java
// CoreNetworkManagementServiceImpl.getElementIp. Only the element types that
// expose a management API are present; others return an error.
var elementLoopbackIP = map[string]string{
	"ims":  "127.0.0.110",
	"amf":  "127.0.0.120",
	"ausf": "127.0.0.130",
	"udm":  "127.0.0.140",
	"smf":  "127.0.0.150",
	"pcf":  "127.0.0.160",
	"upf":  "127.0.0.190",
}

// elementAPIURL builds the core-network element management URL for the given
// API segment (systemManagement or ueManagement) and object type
// (http://<ip>:33030/api/rest/{api}/v1/elementType/{type}/objectType/{objectType}).
// index and extraQuery are mutually exclusive query-string sources.
func elementAPIURL(elementType, api, objectType string, index *int, extraQuery string) (string, error) {
	ip, ok := elementLoopbackIP[elementType]
	if !ok {
		return "", fmt.Errorf("unsupported element type %q (no management endpoint)", elementType)
	}
	url := fmt.Sprintf("http://%s:33030/api/rest/%s/v1/elementType/%s/objectType/%s",
		ip, api, elementType, objectType)
	switch {
	case index != nil:
		url += fmt.Sprintf("?loc=%d", *index)
	case extraQuery != "":
		url += "?" + extraQuery
	}
	return url, nil
}

// callElementAPI performs a REST call against the core-network element's
// management API (systemManagement or ueManagement segment). It returns the
// raw response body.
func callElementAPI(elementType, api, objectType string, index *int, method, extraQuery string, body interface{}) ([]byte, error) {
	url, err := elementAPIURL(elementType, api, objectType, index, extraQuery)
	if err != nil {
		return nil, err
	}
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, url, &reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode >= 400 {
		return buf.Bytes(), fmt.Errorf("element api %s %s returned %d: %s", method, url, resp.StatusCode, buf.String())
	}
	return buf.Bytes(), nil
}

// callElementConfig is a convenience wrapper for the systemManagement segment
// (mirrors the parameter CRUD endpoints).
func callElementConfig(elementType, name string, index *int, method string, body interface{}) ([]byte, error) {
	return callElementAPI(elementType, "systemManagement", name, index, method, "", body)
}
