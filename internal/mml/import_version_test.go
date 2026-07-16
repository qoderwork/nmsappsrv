package mml

import (
	"encoding/xml"
	"testing"
)

// sampleMML mirrors the structure Java's importMMLAndParameter parses:
// /root/hint + type="mml" folders with paraObj commands (params) and nested folders.
const sampleMML = `<?xml version="1.0"?>
<root name="RAN">
  <hint Id="h1" Info="List cell hint"/>
  <mml name="CELL" type="mml">
    <paraObj cmd_label="LST CELL" name="List Cell" help_file="hf1" hint="h1" cmd_type="MTN">
      <param name="LOCALCELLID" id="p1" default="1" relate="" attr="Must" type="int" writable="true" range="1-10"/>
      <param name="STATE" id="p2" default="ON" relate="" attr="Opt" type="enum" writable="false" option="1=ON,2=OFF"/>
    </paraObj>
    <mml name="SUB" type="mml">
      <paraObj cmd_label="LST SUB" name="List Sub" help_file="" hint="" cmd_type="MTN">
        <param name="ID" id="p3" default="" relate="" attr="Must" type="int" writable="true" range="1-5"/>
      </paraObj>
    </mml>
  </mml>
</root>`

// TestMmlImportXMLParsing verifies the XML contract maps to the Go model the
// way Java's VTDNav parsing expects (hintMap, folder/command/param attrs).
func TestMmlImportXMLParsing(t *testing.T) {
	var root mmlImportRoot
	if err := xml.Unmarshal([]byte(sampleMML), &root); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(root.Hints) != 1 || root.Hints[0].Id != "h1" || root.Hints[0].Info != "List cell hint" {
		t.Fatalf("hints mismatch: %+v", root.Hints)
	}
	if len(root.Mmls) != 1 || root.Mmls[0].Type != "mml" || root.Mmls[0].Name != "CELL" {
		t.Fatalf("top folder mismatch: %+v", root.Mmls)
	}

	folder := root.Mmls[0]
	if len(folder.ParaObjs) != 1 {
		t.Fatalf("expected 1 paraObj, got %d", len(folder.ParaObjs))
	}
	po := folder.ParaObjs[0]
	if po.CmdLabel != "LST CELL" || po.CmdType != "MTN" || po.Hint != "h1" || po.HelpFile != "hf1" {
		t.Fatalf("paraObj attr mismatch: %+v", po)
	}
	if len(po.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(po.Params))
	}
	// necessity mapping: attr="Must" -> Necessity=true in Java; raw Attr check here
	if po.Params[0].Attr != "Must" {
		t.Fatalf("LOCALCELLID attr should be Must, got %q", po.Params[0].Attr)
	}
	if po.Params[1].Type != "enum" || po.Params[1].Option != "1=ON,2=OFF" {
		t.Fatalf("STATE should be enum with option: %+v", po.Params[1])
	}
	// nested folder
	if len(folder.Children) != 1 || folder.Children[0].Name != "SUB" {
		t.Fatalf("nested folder mismatch: %+v", folder.Children)
	}
	if len(folder.Children[0].ParaObjs) != 1 || folder.Children[0].ParaObjs[0].CmdLabel != "LST SUB" {
		t.Fatalf("nested paraObj mismatch: %+v", folder.Children[0].ParaObjs)
	}
}

// TestMmlEnumDefaultRemap verifies enum defaultValue remap (option "k=v" → key).
func TestMmlEnumDefaultRemap(t *testing.T) {
	mapped, ok := mmlEnumValueToKey("1=ON,2=OFF", "ON")
	if !ok || mapped != "1" {
		t.Fatalf("expected (1,true), got (%q,%v)", mapped, ok)
	}
	if _, ok := mmlEnumValueToKey("1=ON,2=OFF", "MISSING"); ok {
		t.Fatalf("expected not-found for MISSING")
	}
}

// TestRemoveDuplicateNodes verifies the de-dup contract matches Java's
// removeDuplicateNodesAcrossTree (by CMD:/FOLDER:/NAME: identity).
func TestRemoveDuplicateNodes(t *testing.T) {
	in := []MmlSetVo{
		{Name: "A", IsCommand: true, CommandId: mmlIntPtr(1)},
		{Name: "A", IsCommand: true, CommandId: mmlIntPtr(1)},
		{Name: "B"},
		{Name: "B"},
	}
	out := removeDuplicateNodes(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 unique nodes, got %d", len(out))
	}
}
