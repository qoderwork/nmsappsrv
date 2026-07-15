package mr

import (
	"encoding/xml"
	"io"
	"strings"
	"time"

	"nmsappsrv/pkg/logger"
)

// mrTimeLayout matches Java's DateUtils.parseDate(..., "yyyy-MM-dd'T'HH:mm:ss.SSS").
const mrTimeLayout = "2006-01-02T15:04:05.000"

type bulkFile struct {
	XMLName    xml.Name   `xml:"bulkPmMrDataFile"`
	FileHeader fileHeader `xml:"fileHeader"`
	GNB        []gnb      `xml:"gNB"`
}

type fileHeader struct {
	StartTime string `xml:"startTime,attr"`
	EndTime   string `xml:"endTime,attr"`
}

type gnb struct {
	Measurements []measurement `xml:"measurement"`
}

type measurement struct {
	SMR        string      `xml:"smr"`
	MeasValues []measValue `xml:"measValue"`
}

type measValue struct {
	ID          string `xml:"id,attr"`
	AMFUENGAPID string `xml:"AMFUENGAPID,attr"`
	TimeStamp   string `xml:"TimeStamp,attr"`
	EventType   string `xml:"EventType,attr"`
	Value       string `xml:",chardata"`
}

// ParseMRO decodes an MRO XML document into MRData rows (elementId left 0;
// the caller sets it). Mirrors Java MRFileProcessor.doParseMROFile: per
// measurement, the <smr> header names the 24 MR fields, and each <measValue>
// carries space-separated values aligned to that header.
func ParseMRO(r io.Reader) ([]MRData, error) {
	dec := xml.NewDecoder(r)
	var bf bulkFile
	if err := dec.Decode(&bf); err != nil {
		return nil, err
	}

	startTime := parseMRTime(bf.FileHeader.StartTime)
	endTime := parseMRTime(bf.FileHeader.EndTime)

	var rows []MRData
	for _, g := range bf.GNB {
		for _, m := range g.Measurements {
			names := strings.Fields(m.SMR)
			for _, mv := range m.MeasValues {
				row := MRData{
					StartTime: startTime,
					EndTime:   endTime,
					CellID:    mv.ID,
					UeID:      mv.AMFUENGAPID,
					EventType: mv.EventType,
					EventTime: parseMRTimePtr(mv.TimeStamp),
				}
				values := strings.Fields(mv.Value)
				for i, name := range names {
					if i >= len(values) {
						break
					}
					setMRField(&row, name, values[i])
				}
				rows = append(rows, row)
			}
		}
	}
	return rows, nil
}

func parseMRTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(mrTimeLayout, s)
	if err != nil {
		logger.Warnf("mr: failed to parse header time %q: %v", s, err)
		return nil
	}
	return &t
}

func parseMRTimePtr(s string) *time.Time { return parseMRTime(s) }

// setMRField maps one (mrName, value) pair onto the MRData struct, exactly
// like Java's switch in MRFileProcessor.doParseMROFile.
func setMRField(row *MRData, name, value string) {
	switch name {
	case "MR.NRScArfcn":
		row.NRScArfcn = value
	case "MR.NRScPci":
		row.NRScPci = value
	case "MR.NRScSSRSRP":
		row.NRScSSRSRP = value
	case "MR.NRScSSRSRQ":
		row.NRScSSRSRQ = value
	case "MR.NRScSSSINR":
		row.NRScSSSINR = value
	case "MR.NRScTadv":
		row.NRScTadv = value
	case "MR.NRScPHR":
		row.NRScPHR = value
	case "MR.hAOA":
		row.HAOA = value
	case "MR.vAOA":
		row.VAOA = value
	case "MR.NRUEPlrUL":
		row.NRUEPlrUL = value
	case "MR.NRUEPlrDL":
		row.NRUEPlrDL = value
	case "MR.NRNcArfcn":
		row.NRNcArfcn = value
	case "MR.NRNcPci":
		row.NRNcPci = value
	case "MR.NRNcSSRSRP":
		row.NRNcSSRSRP = value
	case "MR.NRNcSSRSRQ":
		row.NRNcSSRSRQ = value
	case "MR.NRNcSSSINR":
		row.NRNcSSSINR = value
	case "MR.LteNcEarfcn":
		row.LteNcEarfcn = value
	case "MR.LteNcPci":
		row.LteNcPci = value
	case "MR.LteNcRSRP":
		row.LteNcRSRP = value
	case "MR.LteNcRSRQ":
		row.LteNcRSRQ = value
	case "MR.PLMN":
		row.PLMN = value
	case "MR.NRScSSBIndexId":
		row.NRScSSBIndexId = value
	case "MR.NRNcSSBIndexId":
		row.NRNcSSBIndexId = value
	case "MR.Longitude":
		row.Longitude = value
	case "MR.Latitude":
		row.Latitude = value
	default:
		logger.Debugf("mr: unsupported mr name %q", name)
	}
}
