package pmstatistic

import (
	"encoding/csv"
	"os"
	"reflect"
	"strconv"
	"time"
)

type KPIStatisticalDataQuarter struct {
	Id            int64     `json:"id"`
	ElementId     int64     `json:"elementId"`
	KPIName       string    `json:"kpiName"`
	KPIValue      string    `json:"kpiValue"`
	MeasObjLdn    string    `json:"measObjLdn"`
	CellIdentity  string    `json:"cellIdentity"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	PMFileId      int64     `json:"pmFileId"`
	CollectionTime time.Time `json:"collectionTime"`
}

type LDNDTO struct {
	Plmn       string `json:"plmn"`
	LocalNrCI  string `json:"localNrCI"`
	Remote     string `json:"remote"`
}

func ParseLDN(ldn string) *LDNDTO {
	dto := &LDNDTO{}
	pairs := stringsSplit(ldn, ",")
	for _, pair := range pairs {
		kv := stringsSplit(pair, "=")
		if len(kv) == 2 {
			key := stringsTrimSpace(kv[0])
			val := stringsTrimSpace(kv[1])
			switch key {
			case "PLMN":
				dto.Plmn = val
			case "LocalNrCI":
				dto.LocalNrCI = val
			case "Remote":
				dto.Remote = val
			}
		}
	}
	return dto
}

func IsWPS(ldn string) bool {
	return stringsContains(ldn, "WPS")
}

func IsNonWPS(ldn string) bool {
	return !IsWPS(ldn)
}

type GNBResourceUtilization struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	CpuUsage     float64 `csv:"cpuUsage"`
	MemUsage     float64 `csv:"memUsage"`
}

type NRCellCURrcConnection struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	RrcConnAvg   float64 `csv:"rrcConnAvg"`
	RrcConnMax   float64 `csv:"rrcConnMax"`
}

type NRCellCUQosFlow struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	QosFlowAvg   float64 `csv:"qosFlowAvg"`
	QosFlowMax   float64 `csv:"qosFlowMax"`
}

type NRCellCURrcConnectedUserRD struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	RrcConnUsers float64 `csv:"rrcConnUsers"`
}

type NRCellDUPagingRlc struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	PagingRlc    float64 `csv:"pagingRlc"`
}

type NRCellDURlc struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	RlcUlBytes   float64 `csv:"rlcUlBytes"`
	RlcDlBytes   float64 `csv:"rlcDlBytes"`
}

type NRCellDUMacPrb struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	PrbUtilization float64 `csv:"prbUtilization"`
}

type NRCellDUMac struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	MacUlBytes   float64 `csv:"macUlBytes"`
	MacDlBytes   float64 `csv:"macDlBytes"`
}

type NRCellCUUPPdcp struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	PdcpUlBytes  float64 `csv:"pdcpUlBytes"`
	PdcpDlBytes  float64 `csv:"pdcpDlBytes"`
}

type Mobility struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	Remote       string  `csv:"Remote"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	HandoverSucc float64 `csv:"handoverSucc"`
	HandoverAtt  float64 `csv:"handoverAtt"`
}

type PMCellAvailability struct {
	Plmn             string  `csv:"PLMN"`
	LocalNrCI        string  `csv:"LocalNrCI"`
	DeviceName       string  `csv:"deviceName"`
	SerialNumber     string  `csv:"serialNumber"`
	GnbId            string  `csv:"gnbId"`
	GnbName          string  `csv:"gnbName"`
	BeginTime        string  `csv:"beginTime"`
	EndTime          string  `csv:"endTime"`
	CellAvailability float64 `csv:"cellAvailability"`
}

type PMDataVolumeAvailability struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	UlVolume     float64 `csv:"ulVolume"`
	DlVolume     float64 `csv:"dlVolume"`
}

type PMQoSFlow struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	QosFlowCount float64 `csv:"qosFlowCount"`
}

type PMRRC struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	RrcConnCount float64 `csv:"rrcConnCount"`
}

type PMUEContext struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	UeContextAvg float64 `csv:"ueContextAvg"`
	UeContextMax float64 `csv:"ueContextMax"`
}

type PMMobility struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	IntraHoCount float64 `csv:"intraHoCount"`
	InterHoCount float64 `csv:"interHoCount"`
}

type PMThroughput struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	UlThroughput float64 `csv:"ulThroughput"`
	DlThroughput float64 `csv:"dlThroughput"`
}

type Utilization struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	CpuUtil      float64 `csv:"cpuUtil"`
	MemUtil      float64 `csv:"memUtil"`
	RrcUtil      float64 `csv:"rrcUtil"`
}

type NgSignaling struct {
	Plmn         string  `csv:"PLMN"`
	LocalNrCI    string  `csv:"LocalNrCI"`
	DeviceName   string  `csv:"deviceName"`
	SerialNumber string  `csv:"serialNumber"`
	GnbId        string  `csv:"gnbId"`
	GnbName      string  `csv:"gnbName"`
	BeginTime    string  `csv:"beginTime"`
	EndTime      string  `csv:"endTime"`
	NgSigCount   float64 `csv:"ngSigCount"`
}

func ExportCSV[T any](data []T, dirPath, fileName string) error {
	if len(data) == 0 {
		return nil
	}
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}
	filePath := dirPath + "/" + fileName
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	t := reflect.TypeOf(data[0])
	headers := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if csvTag := field.Tag.Get("csv"); csvTag != "" {
			headers = append(headers, csvTag)
		}
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, item := range data {
		values := make([]string, 0, len(headers))
		v := reflect.ValueOf(item)
		for i := 0; i < t.NumField(); i++ {
			field := v.Field(i)
			switch field.Kind() {
			case reflect.String:
				values = append(values, field.String())
			case reflect.Float64:
				values = append(values, strconv.FormatFloat(field.Float(), 'f', -1, 64))
			case reflect.Int, reflect.Int64:
				values = append(values, strconv.FormatInt(field.Int(), 10))
			case reflect.Bool:
				if field.Bool() {
					values = append(values, "true")
				} else {
					values = append(values, "false")
				}
			default:
				values = append(values, "")
			}
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02T15:04:05-07:00")
}

func parseFloat(val string) float64 {
	v, _ := strconv.ParseFloat(val, 64)
	return v
}

func stringsSplit(s, sep string) []string {
	return stringsSplitN(s, sep, -1)
}

func stringsSplitN(s, sep string, n int) []string {
	var result []string
	var current []byte
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			if n > 0 && count >= n-1 {
				result = append(result, string(current))
				current = []byte{}
				count++
				continue
			}
			result = append(result, string(current))
			current = []byte{}
			count++
		} else {
			current = append(current, s[i])
		}
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

func stringsTrimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
