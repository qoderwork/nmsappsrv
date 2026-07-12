package pmfile

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"nmsappsrv/pkg/logger"
)

// ---------- XML structures for 3GPP PM file format (measCollecFile) ----------

// measCollecFile is the root element of a 3GPP PM XML file.
type measCollecFile struct {
	XMLName      xml.Name   `xml:"measCollecFile"`
	FileHeader   fileHeader `xml:"fileHeader"`
	MeasDataList []measData `xml:"measData"`
}

type fileHeader struct {
	MeasCollec measCollec `xml:"measCollec"`
}

type measCollec struct {
	BeginTime string `xml:"beginTime,attr"`
}

type measData struct {
	MeasInfo []measInfo `xml:"measInfo"`
}

type measInfo struct {
	MeasInfoId measInfoId  `xml:"measInfoId"`
	MeasValue  []measValue `xml:"measValue"`
}

type measInfoId struct {
	Name string `xml:",chardata"`
}

type measValue struct {
	MeasObjLdn string   `xml:"measObjLdn,attr"`
	R          []rValue `xml:"r"`
}

type rValue struct {
	P     string `xml:"p,attr"`
	Value string `xml:",chardata"`
}

// ParseXMLFile parses a 3GPP-format PM XML file using the standard library
// xml.Unmarshal. For very large files the V2 streaming approach below is
// preferred, but this covers the common case.
func ParseXMLFile(filePath string) (*PMFileParseResult, error) {
	return ParseXMLFileV2(filePath)
}

// ParseXMLFileV2 reads the entire PM XML file into memory, unmarshals it, and
// extracts KPI measurement values. This mirrors the Java
// ParsePMFileUtil.parseNameAndValueV2 logic.
func ParseXMLFileV2(filePath string) (*PMFileParseResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("pmfile: failed to read file %s: %w", filePath, err)
	}

	var root measCollecFile
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("pmfile: XML unmarshal error: %w", err)
	}

	result := &PMFileParseResult{
		BeginTime: root.FileHeader.MeasCollec.BeginTime,
	}

	// For each measData block, iterate measInfo entries
	for _, md := range root.MeasDataList {
		for _, mi := range md.MeasInfo {
			kpiName := strings.TrimSpace(mi.MeasInfoId.Name)

			for _, mv := range mi.MeasValue {
				cell := extractCellIdentity(mv.MeasObjLdn)
				for _, r := range mv.R {
					val := strings.TrimSpace(r.Value)
					if val == "" {
						continue
					}
					name := kpiName
					if name == "" {
						name = "p" + r.P
					}
					result.KPIs = append(result.KPIs, KPIValue{
						KpiName:      name,
						Value:        val,
						MeasObjLdn:   mv.MeasObjLdn,
						CellIdentity: cell,
					})
				}
			}
		}
	}

	logger.Debugf("pmfile: parsed %d KPI values from file", len(result.KPIs))
	return result, nil
}

// extractCellIdentity extracts the cell identity from a measObjLdn string.
// The LDN format is typically "ManagedElement=xxx,GNBDUFunction=xxx,NRCellDU=CellId"
// or "CellIdentity=xxx". We look for CellIdentity, NRCellDU, or NRCellCU keys.
func extractCellIdentity(ldn string) string {
	if ldn == "" {
		return ""
	}
	parts := strings.Split(ldn, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch strings.ToUpper(key) {
		case "CELLIDENTITY", "NRCELLDU", "NRCELLCU":
			return val
		}
	}
	return ""
}
