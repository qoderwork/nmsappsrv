package parameter

import (
	"testing"
)

// TestBuildDeploySPV verifies that DeployTemplate builds SPV entries from the
// template's DEFINED values (path -> value), with the correct name/value/type.
// This is the core of the P0 fix: we must push defined values, never device
// current values.
func TestBuildDeploySPV(t *testing.T) {
	params := []deployParamValue{
		{ParamPath: "InternetGatewayDevice.DeviceInfo.SerialNumber", ParamValue: "SN-001"},
		{ParamPath: "InternetGatewayDevice.WANDevice.1.WANConnectionDevice.1.WANIPConnection.1.ExternalIPAddress", ParamValue: "10.0.0.5"},
		// multi-instance path must be preserved verbatim
		{ParamPath: "Device.boats.{i}.name", ParamValue: "tug"},
		// empty defined value must be preserved (intentional reset)
		{ParamPath: "Device.empty", ParamValue: ""},
	}

	entries, spv := buildDeploySPV(params)

	if len(entries) != 4 || len(spv) != 4 {
		t.Fatalf("expected 4 entries, got entries=%d spv=%d", len(entries), len(spv))
	}

	for i, p := range params {
		if entries[i].ParamName != p.ParamPath {
			t.Errorf("entry[%d].ParamName = %q, want %q", i, entries[i].ParamName, p.ParamPath)
		}
		if entries[i].ParamValue != p.ParamValue {
			t.Errorf("entry[%d].ParamValue = %q, want %q", i, entries[i].ParamValue, p.ParamValue)
		}
		if spv[i].Name != p.ParamPath {
			t.Errorf("spv[%d].Name = %q, want %q", i, spv[i].Name, p.ParamPath)
		}
		if spv[i].Value != p.ParamValue {
			t.Errorf("spv[%d].Value = %q, want %q", i, spv[i].Value, p.ParamValue)
		}
		if spv[i].Type != "xsd:string" {
			t.Errorf("spv[%d].Type = %q, want xsd:string", i, spv[i].Type)
		}
	}
}

// TestToTemplateParamRows verifies the defined (target) value is persisted on
// the association row, keyed by template_id + parameter_id.
func TestToTemplateParamRows(t *testing.T) {
	params := []TemplateParameter{
		{ParameterId: "param-uuid-a", Value: "value-a"},
		{ParameterId: "param-uuid-b", Value: ""},
	}
	rows := toTemplateParamRows(42, params)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	if rows[0].TemplateId == nil || *rows[0].TemplateId != 42 {
		t.Errorf("row[0].TemplateId = %v, want 42", rows[0].TemplateId)
	}
	if rows[0].ParameterId == nil || *rows[0].ParameterId != "param-uuid-a" {
		t.Errorf("row[0].ParameterId = %v, want param-uuid-a", rows[0].ParameterId)
	}
	if rows[0].ParameterValue == nil || *rows[0].ParameterValue != "value-a" {
		t.Errorf("row[0].ParameterValue = %v, want value-a", rows[0].ParameterValue)
	}
	if rows[1].ParameterValue == nil || *rows[1].ParameterValue != "" {
		t.Errorf("row[1].ParameterValue = %v, want empty string", rows[1].ParameterValue)
	}
}
