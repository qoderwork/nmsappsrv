package corenet

import (
	"embed"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed paramconfig/*.yaml
var paramConfigFS embed.FS

// ---------------------------------------------------------------------------
// Response / request structures for core-network parameter management.
// These mirror the Java CoreNetworkParamElementVO / CoreNetworkParamVO /
// CoreNetworkParamDTO and the *CoreNetworkParameter(s)DTO request objects.
// ---------------------------------------------------------------------------

// CoreNetworkParam mirrors Java CoreNetworkParamDTO.CoreNetworkParam.
type CoreNetworkParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Writable    bool   `json:"writable"`
	DisplayName string `json:"displayName"`
	Filter      string `json:"filter"`
	Comment     string `json:"comment"`
}

// CoreNetworkParamVO mirrors Java CoreNetworkParamVO (one parameter set).
type CoreNetworkParamVO struct {
	Table                bool              `json:"table"`
	Name                 string            `json:"name"`
	DisplayName          string            `json:"displayName"`
	ParameterInformation []CoreNetworkParam `json:"parameterInformation"`
	Data                 []map[string]interface{} `json:"data"`
}

// CoreNetworkParamElementVO mirrors Java CoreNetworkParamElementVO.
type CoreNetworkParamElementVO struct {
	ElementType string               `json:"elementType"`
	Params      []CoreNetworkParamVO `json:"params"`
}

// GetCoreNetworkParametersQuery mirrors Java GetCoreNetworkParametersQuery.
type GetCoreNetworkParametersQuery struct {
	CoreNetworkId int    `json:"coreNetworkId"`
	ElementType   string `json:"elementType"`
}

// SetCoreNetworkParametersDTO mirrors Java SetCoreNetworkParametersDTO /
// AddCoreNetworkParameter (shares the same shape).
type SetCoreNetworkParametersDTO struct {
	CoreNetworkId int                    `json:"coreNetworkId"`
	Name          string                 `json:"name"`
	Index         *int                   `json:"index"`
	Data          map[string]interface{} `json:"data"`
	ElementType   string                 `json:"elementType"`
}

// QueryCoreNetworkParametersDTO mirrors Java QueryCoreNetworkParametersDTO.
type QueryCoreNetworkParametersDTO struct {
	Name          string `json:"name"`
	CoreNetworkId int    `json:"coreNetworkId"`
	ElementType   string `json:"elementType"`
}

// DeleteCoreNetworkParameterDTO mirrors Java DeleteCoreNetworkParameterDTO.
type DeleteCoreNetworkParameterDTO struct {
	CoreNetworkId int    `json:"coreNetworkId"`
	Name          string `json:"name"`
	Index         int    `json:"index"`
	ElementType   string `json:"elementType"`
}

// ---------------------------------------------------------------------------
// Catalog loader (mirrors Java StationRunner.loadCoreNetworkParamConfig).
// The catalog is loaded from classpath YAML; the top-level key is the
// elementType, each nested entry is a parameter set.
// ---------------------------------------------------------------------------

type rawCatalog map[string]map[string]rawSet

type rawSet struct {
	Display string     `yaml:"display"`
	List    []rawParam `yaml:"list"`
	Array   interface{} `yaml:"array"`
}

type rawParam struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Value   string `yaml:"value"`
	Access  string `yaml:"access"`
	Filter  string `yaml:"filter"`
	Display string `yaml:"display"`
	Comment string `yaml:"comment"`
}

// loadCatalog reads and parses the parameter catalog for an element type.
func loadCatalog(elementType string) (*CoreNetworkParamElementVO, error) {
	path := fmt.Sprintf("paramconfig/%s_param_config.yaml", elementType)
	b, err := paramConfigFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("param config not found for element type %q", elementType)
	}
	var raw rawCatalog
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	// The top-level key is the element type; fall back to the first key.
	setMap, ok := raw[elementType]
	if !ok {
		for k, v := range raw {
			elementType, setMap, ok = k, v, true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("empty param config for element type %q", elementType)
	}
	vo := &CoreNetworkParamElementVO{ElementType: elementType}
	for setName, set := range setMap {
		pvo := CoreNetworkParamVO{
			Table:       set.Array != nil,
			Name:        setName,
			DisplayName: set.Display,
		}
		for _, rp := range set.List {
			pvo.ParameterInformation = append(pvo.ParameterInformation, CoreNetworkParam{
				Name:        rp.Name,
				Type:        rp.Type,
				Writable:    strings.EqualFold(rp.Access, "read-write"),
				DisplayName: rp.Display,
				Filter:      rp.Filter,
				Comment:     rp.Comment,
			})
		}
		vo.Params = append(vo.Params, pvo)
	}
	return vo, nil
}

// titleCase capitalises only the first letter (used to build the
// Config{ElementType}{Set} CoreNetworkData column name).
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// enrichCoreNetParamData attaches the stored config values (from
// core_network_data) to a parameter set, mirroring Java
// getNameAndValueByElementTypeAndParamName. The column follows the
// Config{ElementType}{Set} naming convention.
func enrichCoreNetParamData(data *CoreNetworkData, elementType, setName string) []map[string]interface{} {
	if data == nil {
		return nil
	}
	fieldName := fmt.Sprintf("Config%s%s", titleCase(elementType), titleCase(setName))
	rv := reflect.ValueOf(*data)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	f := rv.FieldByName(fieldName)
	if !f.IsValid() {
		return nil
	}
	pstr, ok := f.Interface().(*string)
	if !ok || pstr == nil || *pstr == "" {
		return nil
	}
	return parseConfigJSON(*pstr)
}

// parseConfigJSON converts a stored config JSON string into a list of
// name/value maps. Objects become one entry per key; arrays of objects pass
// through.
func parseConfigJSON(s string) []map[string]interface{} {
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(s), &arr); err == nil {
		return arr
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(s), &obj); err == nil {
		out := make([]map[string]interface{}, 0, len(obj))
		for k, v := range obj {
			out = append(out, map[string]interface{}{"name": k, "value": v})
		}
		return out
	}
	return nil
}
