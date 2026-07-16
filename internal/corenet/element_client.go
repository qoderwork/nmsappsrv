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

// elementConfigURL builds the core-network element management URL
// (http://<ip>:33030/api/rest/systemManagement/v1/elementType/{type}/objectType/config/{name}).
func elementConfigURL(elementType, name string, index *int) (string, error) {
	ip, ok := elementLoopbackIP[elementType]
	if !ok {
		return "", fmt.Errorf("unsupported element type %q (no management endpoint)", elementType)
	}
	url := fmt.Sprintf("http://%s:33030/api/rest/systemManagement/v1/elementType/%s/objectType/config/%s",
		ip, elementType, name)
	if index != nil {
		url += fmt.Sprintf("?loc=%d", *index)
	}
	return url, nil
}

// callElementConfig performs a REST call against the core-network element's
// management API (mirrors Java building an HttpRequest with PUT/GET/DELETE/
// POST to the 33030 endpoint). It returns the raw response body.
func callElementConfig(elementType, name string, index *int, method string, body interface{}) ([]byte, error) {
	url, err := elementConfigURL(elementType, name, index)
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
		return buf.Bytes(), fmt.Errorf("element config %s %s returned %d: %s", method, url, resp.StatusCode, buf.String())
	}
	return buf.Bytes(), nil
}
