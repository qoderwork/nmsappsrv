package misc

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"nmsappsrv/pkg/logger"
)

// ---------- 17 xlsm DTOs (§A) ----------
// Every data sheet DTO embeds No (1-based data-row order) which drives the
// instance-numbering algorithms in §C. All other fields mirror the spec
// column indices exactly; the Go field name is irrelevant, only the column
// index used during parsing matters.

type DeviceInfoDTO struct {
	No           int
	GndId        string
	DeviceName   string
	SerialNumber string
	Longitude    string
	Latitude     string
	AreaCode     string
	Location     string
	DeviceGroup  string
	SiteCode     string
}

type BaseStationIdentifierDTO struct {
	No          int
	GndId       string
	GnbName     string
	GnbIdLength string
}

type NTPServerInfoDTO struct {
	No            int
	GndId         string
	ClockSource   string
	Enable        string
	NtpServer1    string
	NtpServer2    string
	NtpServer3    string
	NtpServer4    string
	NtpServer5    string
	LocalTimeZone string
}

type AMFConfigDTO struct {
	No                 int
	GndId              string
	Ip1                string
	Ip2                string
	LocalUpIPAddress   string
	LocalUpIPV6Address string
	LocalCpIPAddress   string
	LocalCpIPV6Address string
	PlmnId             string
	AmfPort            string
	AdminState         string
}

type VlanInterfaceConfigDTO struct {
	No                 int
	GndId              string
	Id                 string
	Enable             string
	IPAddress          string
	SubnetMask         string
	AddressingType     string
	DefaultGateway     string
	PortType           string
	IPv6Address        string
	IpV6PrefixLength   string
	IpV6Origin         string
	IpV6DefaultGateway string
	IpV6PortType       string
}

type RouteConfigDTO struct {
	No               int
	GndId            string
	IpVersion        string
	DstIpNetwork     string
	PrefixLength     string
	GatewayIpAddress string
	InterfaceName    string
}

type CellBasicDTO struct {
	No                              int
	GndId                           string
	CellIdWithinGnb                 string
	NRARFCNUL                       string
	NRARFCNDL                       string
	DLBandwidth                     string
	ULBandwidth                     string
	Pci                             string
	SsbFrequency                    string
	CSRS1                           string
	CSRS2                           string
	RbNumber                        string
	AbsoluteFrequencySSB            string
	Pattern2Present                 string
	Pattern1TransmissionPeriodicity string
	Patter1NrofDownlinkSymbols      string
	Patter1NrofDownlinkSlots        string
	Patter1NrofUplinSymbols         string
	Patter1NrofUplinkSlots          string
	Pattern2TransmissionPeriodicity string
	Patter2NrofDownlinkSymbols      string
	Patter2NrofDownlinkSlots        string
	Patter2NrofUplinSymbols         string
	Patter2NrofUplinkSlots          string
	FreqBandIndicator               string
	EnableSliceAccessControl        string
	SliceEnable                     string
	SliceFreeAtIdleEnable           string
	QosHeavyControl                 string
	RouteIndexList                  string
	RuList                          string
}

type CellAdvancedDTO struct {
	No              int
	GndId           string
	CellIdWithinGnb string
	Tac             string
	Ransc           string // §B.8 source "ransc" -> .Ranac (extra column beyond §A.8)
}

type QoSDTO struct {
	No              int
	GndId           string
	CellIdWithinGnb string
	Index           string
	FiveQI          string
	PLMNID          string
	RlcMode         string
	SNSSAI          string
}

type VoNRDTO struct {
	No              int
	GndId           string
	CellIdWithinGnb string
	PlmnId          string
	Vo5qi           string
	VoSupport       string
}

type PLMNForCellDTO struct {
	No              int
	GndId           string
	CellIdWithinGnb string
	Tac             string
	PLMN            string
}

type SliceForCellDTO struct {
	No              int
	GndId           string
	CellIdWithinGnb string
	Index           string
	SliceFreeAtIdle string
	Heavy           string
}

type FloatForCellDTO struct {
	No              int
	GndId           string
	CellIdWithinGnb string
	Heavy           string
}

type SliceForPlmnDTO struct {
	No                  int
	GndId               string
	CellIdWithinGnb     string
	Tac                 string
	PLMN                string
	SNSSAI              string
	RrcCountWithinNssai string
}

type IPSecDTO struct {
	No                           int
	GndId                        string
	Enable                       string
	AuthenticationMethod         string
	PreSharedKey                 string
	Certs                        string
	SecGWServer1                 string
	SecGWServer2                 string
	LocalId                      string
	RemoteId                     string
	LocalPort                    string
	LocalNattPort                string
	RemotePort                   string
	LocalEapId                   string
	RemoteEapId                  string
	Opc                          string
	K                            string
	EncryptionAlgorithms         string
	IntegrityAlgorithms          string
	DiffieHellmanGroupTransforms string
	EnableVips                   string
	EnableVipsV6                 string
	DpdDelay                     string
	Recovery                     string
	Index                        string
}

type ChildSADTO struct {
	No                           int
	GndId                        string
	IpsecIndex                   string
	ChildSAIndex                 string
	Id                           string
	LocalTs                      string
	RemoteTs                     string
	EncryptionAlgorithms         string
	IntegrityAlgorithms          string
	DiffieHellmanGroupTransforms string
}

type ClockDTO struct {
	No       int
	GndId    string
	SyncMode string
}

// ---------- parsed container + sheet names ----------

type nrAOSData struct {
	deviceInfo   []DeviceInfoDTO
	baseId       []BaseStationIdentifierDTO
	ntp          []NTPServerInfoDTO
	amf          []AMFConfigDTO
	vlan         []VlanInterfaceConfigDTO
	route        []RouteConfigDTO
	cellBasic    []CellBasicDTO
	cellAdvanced []CellAdvancedDTO
	qos          []QoSDTO
	vonr         []VoNRDTO
	plmnForCell  []PLMNForCellDTO
	sliceForCell []SliceForCellDTO
	floatForCell []FloatForCellDTO
	sliceForPLMN []SliceForPlmnDTO
	ipSec        []IPSecDTO
	childSA      []ChildSADTO
	clock        []ClockDTO
}

var nrAOSRequiredSheets = []string{
	"Device Info",
	"Basic Identification",
	"NTP",
	"AMF",
	"Network Interface",
	"Route",
	"NR Cell Basic Parameter",
	"NR Cell Advanced Parameter",
	"QoS",
	"VoNR",
	"PLMN For Cell",
	"Slice For Cell",
	"Float For Cell",
	"Slice For PLMN",
	"IP Sec",
	"Child SA",
	"Clock",
	"Instructions for Use",
}

// ---------- validation error ----------

type aosVerifyError struct {
	Code int
	Msg  string
}

func (e *aosVerifyError) Error() string {
	return fmt.Sprintf("AOS template validation failed (code %d): %s", e.Code, e.Msg)
}

func verr(code int, msg string) error {
	return &aosVerifyError{Code: code, Msg: msg}
}

// ---------- helpers ----------

func cellVal(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

var localTimeZoneRe = regexp.MustCompile(`^UTC[+-][0-9][0-9]$`)

func isIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

// enum maps (§B)
var clockSourceMap = map[string]string{
	"GPS":                 "1",
	"GLONASS":             "2",
	"BDS":                 "3",
	"1588v2":              "4",
	"GPS/GLONASS":         "5",
	"BDS/GPS":             "6",
	"GPS/BDS":             "7",
	"GALILEO":             "8",
	"GPS/GALILEO":         "9",
	"GPS/GALILEO/GLONASS": "10",
	"GPS/GALILEO/BDS":     "11",
}

var patternPeriodicityMap = map[string]string{
	"ms0p5":   "0",
	"ms0p625": "1",
	"ms1":     "2",
	"ms1p25":  "3",
	"ms2":     "4",
	"ms2p5":   "5",
	"ms3":     "6",
	"ms4":     "7",
	"ms5":     "8",
	"ms10":    "9",
}

func mapOrEmpty(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return ""
}

func boolTrue(s string) string {
	if s == "Enabled" {
		return "true"
	}
	return "false"
}

func enabledTo1(s string) string {
	if s == "Enabled" {
		return "1"
	}
	return "0"
}

func enableToBool(s string) string {
	if s == "Enable" {
		return "true"
	}
	return "false"
}

func ipv4ToNum(s string) string {
	if s == "IPV4" {
		return "1"
	}
	return "2"
}

func cellKey(gndId, cellId string) string     { return gndId + ":" + cellId }
func tacKey(gndId, cellId, tac string) string { return gndId + ":" + cellId + ":" + tac }
func plmnKey(gndId, cellId, tac, plmn string) string {
	return gndId + ":" + cellId + ":" + tac + ":" + plmn
}

// ---------- parse ----------

// ImportNrAOSFile parses an xlsm template, validates it, upserts devices and
// their groups, generates the per-device AOS parameter set, writes it to
// cpe_element.ztp_parameters, generates the AOS pull-file (without flipping
// read_to_ztp) and records the gnbId allocation. It mirrors Java's
// AOSManagementServiceImpl.importNrAOSFile. Returns the taskId (for polling
// progress via getGenerateAOSFileTaskProgress), the number of devices
// processed, and any error.
func (s *service) ImportNrAOSFile(filePath string, tenantId int) (taskId string, count int, err error) {
	taskId = strings.ReplaceAll(uuid.New().String(), "-", "")
	data, err := parseNrAOSFile(filePath)
	if err != nil {
		s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
		return taskId, 0, err
	}

	if err := s.verifyData(data); err != nil {
		s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
		return taskId, 0, err
	}

	// 3. Device groups: collect non-empty deviceGroup values from Device Info.
	gndIdToGroup := make(map[string]string)
	for _, d := range data.deviceInfo {
		if d.DeviceGroup == "" {
			continue
		}
		gndIdToGroup[d.GndId] = d.DeviceGroup
	}
	groupNameToId := make(map[string]string)
	for _, name := range uniqueStrings(collectDeviceGroups(data.deviceInfo)) {
		id, err := s.upsertDeviceGroup(name, tenantId)
		if err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("upsert device group %q: %w", name, err)
		}
		groupNameToId[name] = id
	}

	// 4/5. Build serialNumber -> gnbId and upsert cpe_element.
	serialToGnb := make(map[string]string)
	for _, d := range data.deviceInfo {
		if d.SerialNumber == "" {
			continue
		}
		serialToGnb[d.SerialNumber] = d.GndId
	}

	serialToId := make(map[string]int64)
	for serial, gnbId := range serialToGnb {
		groupName := gndIdToGroup[gnbId]
		groupId := groupNameToId[groupName] // "" if none
		areaCode := ""
		for _, d := range data.deviceInfo {
			if d.SerialNumber == serial {
				areaCode = d.AreaCode
				break
			}
		}
		elementId, err := s.upsertCpeElement(serial, tenantId, areaCode, groupId)
		if err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("upsert device %s: %w", serial, err)
		}
		serialToId[serial] = elementId
	}

	// 6. Group DTOs by gndId.
	cellBasicMap, cellAdvancedMap, qosMap, vonrMap, plmnForCellMap, sliceForCellMap,
		floatForCellMap, sliceForPLMNMap, amfMap, vlanMap, routeMap, ntpMap,
		identifierMap, clockMap, ipSecMap, childSAMap := groupByGndId(data)

	// 9. Instance numbering (§C).
	cellIdMap, tacMap, plmnMap := buildInstanceNumbers(cellBasicMap, cellAdvancedMap, plmnForCellMap)

	// Initial progress: complete=false, 0 / total.
	total := len(serialToGnb)
	s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{
		Complete:        false,
		CurrentProgress: 0,
		TotalProgress:   total,
		Message:         "import started",
	})

	// 10. Per-device generation.
	count = 0
	for serial, gnbId := range serialToGnb {
		elementId := serialToId[serial]
		gen := &aosGenerator{
			gndId:           gnbId,
			cellBasicMap:    cellBasicMap,
			cellAdvancedMap: cellAdvancedMap,
			qosMap:          qosMap,
			vonrMap:         vonrMap,
			plmnForCellMap:  plmnForCellMap,
			sliceForCellMap: sliceForCellMap,
			sliceForPLMNMap: sliceForPLMNMap,
			floatForCellMap: floatForCellMap,
			amfMap:          amfMap,
			vlanMap:         vlanMap,
			routeMap:        routeMap,
			ntpMap:          ntpMap,
			identifierMap:   identifierMap,
			clockMap:        clockMap,
			ipSecMap:        ipSecMap,
			childSAMap:      childSAMap,
			cellIdMap:       cellIdMap,
			tacMap:          tacMap,
			plmnMap:         plmnMap,
			params:          make(map[string]string),
		}
		gen.run()

		b, err := json.Marshal(gen.params)
		if err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("marshal params for device %s: %w", serial, err)
		}
		if err := s.repo.DB().Table("cpe_element").
			Where("ne_neid = ?", elementId).
			Update("ztp_parameters", string(b)).Error; err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("store params for device %s: %w", serial, err)
		}

		if _, err := s.GenerateAOSFile(elementId, false); err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("generate AOS file for device %s: %w", serial, err)
		}
		if err := s.repo.DeleteZTPLogsByElementIds([]int64{elementId}); err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("delete ztp logs for device %s: %w", serial, err)
		}

		gnbInt, err := strconv.Atoi(gnbId)
		if err != nil {
			gnbInt = 0
		}
		rec := ZTPGnbIdUsed{
			Id:        strings.ReplaceAll(uuid.New().String(), "-", ""),
			ElementId: int64Ptr(elementId),
			Market:    strPtr("MANUAL"),
			GnbId:     &gnbInt,
		}
		if err := s.repo.DB().Create(&rec).Error; err != nil {
			s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{HasFault: true, Message: err.Error()})
			return taskId, 0, fmt.Errorf("record gnbId for device %s: %w", serial, err)
		}
		count++
		s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{
			Complete:        false,
			CurrentProgress: count,
			TotalProgress:   total,
			Message:         fmt.Sprintf("processing device %d/%d", count, total),
		})
	}

	s.flushAOSTaskProgress(taskId, &AOSTaskProgressVO{
		Complete:        true,
		CurrentProgress: total,
		TotalProgress:   total,
		Message:         "import completed",
	})
	logger.Infof("NR AOS import completed: task=%s, %d devices processed from %s", taskId, count, filePath)
	return taskId, count, nil
}

func (s *service) upsertDeviceGroup(name string, tenantId int) (string, error) {
	var id string
	if err := s.repo.DB().Table("device_group").
		Select("id").Where("group_name = ? AND license_id = ?", name, tenantId).
		Scan(&id).Error; err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	id = strings.ReplaceAll(uuid.New().String(), "-", "")
	now := time.Now()
	err := s.repo.DB().Table("device_group").Create(map[string]interface{}{
		"id":            id,
		"group_name":    name,
		"license_id":     tenantId,
		"creation_time": now,
		"default_group": false,
	}).Error
	return id, err
}

// cpeElementUpsert is a minimal projection of cpe_element used only for the
// import upsert (misc must not import the device package).
type cpeElementUpsert struct {
	NeNeid        int64   `gorm:"primaryKey;autoIncrement;column:ne_neid"`
	SerialNumber  *string `gorm:"column:serial_number"`
	DeviceType    *string `gorm:"column:device_type"`
	Generation    *string `gorm:"column:generation"`
	TenantId      *int    `gorm:"column:license_id"`
	NeAreaid      *int    `gorm:"column:ne_areaid"`
	DeviceGroupId *string `gorm:"column:device_group_id"`
	Deleted       bool    `gorm:"column:deleted"`
}

func (cpeElementUpsert) TableName() string { return "cpe_element" }

func (s *service) upsertCpeElement(serial string, tenantId int, areaCode, deviceGroupId string) (int64, error) {
	var el cpeElementUpsert
	err := s.repo.DB().Table("cpe_element").
		Select("ne_neid").Where("serial_number = ? AND deleted = ?", serial, false).
		First(&el).Error
	if err == gorm.ErrRecordNotFound {
		row := cpeElementUpsert{
			SerialNumber:  &serial,
			DeviceType:    strPtr("enb"),
			Generation:    strPtr("NR"),
			TenantId:      &tenantId,
			DeviceGroupId: nilIfEmpty(deviceGroupId),
			Deleted:       false,
		}
		if v, ok := parseInt(areaCode); ok {
			row.NeAreaid = &v
		}
		if err := s.repo.DB().Table("cpe_element").Create(&row).Error; err != nil {
			return 0, err
		}
		return row.NeNeid, nil
	}
	if err != nil {
		return 0, err
	}
	updates := map[string]interface{}{}
	if v, ok := parseInt(areaCode); ok {
		updates["ne_areaid"] = v
	}
	if deviceGroupId != "" {
		updates["device_group_id"] = deviceGroupId
	}
	if len(updates) > 0 {
		if err := s.repo.DB().Table("cpe_element").
			Where("ne_neid = ?", el.NeNeid).Updates(updates).Error; err != nil {
			return 0, err
		}
	}
	return el.NeNeid, nil
}

func parseNrAOSFile(filePath string) (*nrAOSData, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("open xlsm: %w", err)
	}
	defer f.Close()

	// 2. validate all required sheet names exist.
	present := make(map[string]bool)
	for _, name := range f.GetSheetList() {
		present[name] = true
	}
	for _, name := range nrAOSRequiredSheets {
		if !present[name] {
			return nil, fmt.Errorf("invalid template: missing sheet %q", name)
		}
	}

	d := &nrAOSData{}

	// 16 sheets with header at row 0, data from row 1.
	if err := parseSheet(f, "Device Info", 1, d, func(r []string, no int) {
		d.deviceInfo = append(d.deviceInfo, DeviceInfoDTO{
			No: no, GndId: cellVal(r, 0), DeviceName: cellVal(r, 1),
			SerialNumber: cellVal(r, 2), Longitude: cellVal(r, 3),
			Latitude: cellVal(r, 4), AreaCode: cellVal(r, 5),
			Location: cellVal(r, 6), DeviceGroup: cellVal(r, 7), SiteCode: cellVal(r, 8),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Basic Identification", 1, d, func(r []string, no int) {
		d.baseId = append(d.baseId, BaseStationIdentifierDTO{
			No: no, GndId: cellVal(r, 0), GnbName: cellVal(r, 1), GnbIdLength: cellVal(r, 2),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "NTP", 1, d, func(r []string, no int) {
		d.ntp = append(d.ntp, NTPServerInfoDTO{
			No: no, GndId: cellVal(r, 0), ClockSource: cellVal(r, 1), Enable: cellVal(r, 2),
			NtpServer1: cellVal(r, 3), NtpServer2: cellVal(r, 4), NtpServer3: cellVal(r, 5),
			NtpServer4: cellVal(r, 6), NtpServer5: cellVal(r, 7), LocalTimeZone: cellVal(r, 8),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "AMF", 1, d, func(r []string, no int) {
		d.amf = append(d.amf, AMFConfigDTO{
			No: no, GndId: cellVal(r, 0), Ip1: cellVal(r, 1), Ip2: cellVal(r, 2),
			LocalUpIPAddress: cellVal(r, 3), LocalUpIPV6Address: cellVal(r, 4),
			LocalCpIPAddress: cellVal(r, 5), LocalCpIPV6Address: cellVal(r, 6),
			PlmnId: cellVal(r, 7), AmfPort: cellVal(r, 8), AdminState: cellVal(r, 9),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Network Interface", 1, d, func(r []string, no int) {
		d.vlan = append(d.vlan, VlanInterfaceConfigDTO{
			No: no, GndId: cellVal(r, 0), Id: cellVal(r, 1), Enable: cellVal(r, 2),
			IPAddress: cellVal(r, 3), SubnetMask: cellVal(r, 4), AddressingType: cellVal(r, 5),
			DefaultGateway: cellVal(r, 6), PortType: cellVal(r, 7), IPv6Address: cellVal(r, 8),
			IpV6PrefixLength: cellVal(r, 9), IpV6Origin: cellVal(r, 10),
			IpV6DefaultGateway: cellVal(r, 11), IpV6PortType: cellVal(r, 12),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Route", 1, d, func(r []string, no int) {
		d.route = append(d.route, RouteConfigDTO{
			No: no, GndId: cellVal(r, 0), IpVersion: cellVal(r, 1), DstIpNetwork: cellVal(r, 2),
			PrefixLength: cellVal(r, 3), GatewayIpAddress: cellVal(r, 4), InterfaceName: cellVal(r, 5),
		})
	}); err != nil {
		return nil, err
	}
	// NR Cell Basic Parameter: headRowNumber=2 -> header at row 2, data from row 3.
	if err := parseSheet(f, "NR Cell Basic Parameter", 3, d, func(r []string, no int) {
		d.cellBasic = append(d.cellBasic, CellBasicDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1),
			NRARFCNUL: cellVal(r, 2), NRARFCNDL: cellVal(r, 3), DLBandwidth: cellVal(r, 4),
			ULBandwidth: cellVal(r, 5), Pci: cellVal(r, 6), SsbFrequency: cellVal(r, 7),
			CSRS1: cellVal(r, 8), CSRS2: cellVal(r, 9), RbNumber: cellVal(r, 10),
			AbsoluteFrequencySSB: cellVal(r, 11), Pattern2Present: cellVal(r, 12),
			Pattern1TransmissionPeriodicity: cellVal(r, 13), Patter1NrofDownlinkSymbols: cellVal(r, 14),
			Patter1NrofDownlinkSlots: cellVal(r, 15), Patter1NrofUplinSymbols: cellVal(r, 16),
			Patter1NrofUplinkSlots: cellVal(r, 17), Pattern2TransmissionPeriodicity: cellVal(r, 18),
			Patter2NrofDownlinkSymbols: cellVal(r, 19), Patter2NrofDownlinkSlots: cellVal(r, 20),
			Patter2NrofUplinSymbols: cellVal(r, 21), Patter2NrofUplinkSlots: cellVal(r, 22),
			FreqBandIndicator: cellVal(r, 23), EnableSliceAccessControl: cellVal(r, 24),
			SliceEnable: cellVal(r, 25), SliceFreeAtIdleEnable: cellVal(r, 26),
			QosHeavyControl: cellVal(r, 27), RouteIndexList: cellVal(r, 28), RuList: cellVal(r, 29),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "NR Cell Advanced Parameter", 1, d, func(r []string, no int) {
		d.cellAdvanced = append(d.cellAdvanced, CellAdvancedDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), Tac: cellVal(r, 2),
			Ransc: cellVal(r, 3),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "QoS", 1, d, func(r []string, no int) {
		d.qos = append(d.qos, QoSDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), Index: cellVal(r, 2),
			FiveQI: cellVal(r, 3), PLMNID: cellVal(r, 4), RlcMode: cellVal(r, 5), SNSSAI: cellVal(r, 6),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "VoNR", 1, d, func(r []string, no int) {
		d.vonr = append(d.vonr, VoNRDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), PlmnId: cellVal(r, 2),
			Vo5qi: cellVal(r, 3), VoSupport: cellVal(r, 4),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "PLMN For Cell", 1, d, func(r []string, no int) {
		d.plmnForCell = append(d.plmnForCell, PLMNForCellDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), Tac: cellVal(r, 2),
			PLMN: cellVal(r, 3),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Slice For Cell", 1, d, func(r []string, no int) {
		d.sliceForCell = append(d.sliceForCell, SliceForCellDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), Index: cellVal(r, 2),
			SliceFreeAtIdle: cellVal(r, 3), Heavy: cellVal(r, 4),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Float For Cell", 1, d, func(r []string, no int) {
		d.floatForCell = append(d.floatForCell, FloatForCellDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), Heavy: cellVal(r, 2),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Slice For PLMN", 1, d, func(r []string, no int) {
		d.sliceForPLMN = append(d.sliceForPLMN, SliceForPlmnDTO{
			No: no, GndId: cellVal(r, 0), CellIdWithinGnb: cellVal(r, 1), Tac: cellVal(r, 2),
			PLMN: cellVal(r, 3), SNSSAI: cellVal(r, 4), RrcCountWithinNssai: cellVal(r, 5),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "IP Sec", 1, d, func(r []string, no int) {
		d.ipSec = append(d.ipSec, IPSecDTO{
			No: no, GndId: cellVal(r, 0), Enable: cellVal(r, 1), AuthenticationMethod: cellVal(r, 2),
			PreSharedKey: cellVal(r, 3), Certs: cellVal(r, 4), SecGWServer1: cellVal(r, 5),
			SecGWServer2: cellVal(r, 6), LocalId: cellVal(r, 7), RemoteId: cellVal(r, 8),
			LocalPort: cellVal(r, 9), LocalNattPort: cellVal(r, 10), RemotePort: cellVal(r, 11),
			LocalEapId: cellVal(r, 12), RemoteEapId: cellVal(r, 13), Opc: cellVal(r, 14),
			K: cellVal(r, 15), EncryptionAlgorithms: cellVal(r, 16), IntegrityAlgorithms: cellVal(r, 17),
			DiffieHellmanGroupTransforms: cellVal(r, 18), EnableVips: cellVal(r, 19),
			EnableVipsV6: cellVal(r, 20), DpdDelay: cellVal(r, 21), Recovery: cellVal(r, 22),
			Index: cellVal(r, 23),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Child SA", 1, d, func(r []string, no int) {
		d.childSA = append(d.childSA, ChildSADTO{
			No: no, GndId: cellVal(r, 0), IpsecIndex: cellVal(r, 1), ChildSAIndex: cellVal(r, 2),
			Id: cellVal(r, 3), LocalTs: cellVal(r, 4), RemoteTs: cellVal(r, 5),
			EncryptionAlgorithms: cellVal(r, 6), IntegrityAlgorithms: cellVal(r, 7),
			DiffieHellmanGroupTransforms: cellVal(r, 8),
		})
	}); err != nil {
		return nil, err
	}
	if err := parseSheet(f, "Clock", 1, d, func(r []string, no int) {
		d.clock = append(d.clock, ClockDTO{
			No: no, GndId: cellVal(r, 0), SyncMode: cellVal(r, 1),
		})
	}); err != nil {
		return nil, err
	}

	return d, nil
}

// parseSheet reads rows from a sheet, skipping the header (and, for the Cell
// Basic sheet, the two leading non-data rows). Rows whose gndId (col 0) is
// empty are discarded. Each kept row is passed to cb with its 1-based data-row
// number (No).
func parseSheet(f *excelize.File, sheet string, dataStart int, _ *nrAOSData,
	cb func(row []string, no int)) error {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return fmt.Errorf("read sheet %q: %w", sheet, err)
	}
	no := 0
	for i := dataStart; i < len(rows); i++ {
		row := rows[i]
		if cellVal(row, 0) == "" {
			continue
		}
		no++
		cb(row, no)
	}
	return nil
}

// ---------- grouping + instance numbering ----------

func groupByGndId(d *nrAOSData) (
	cellBasicMap map[string][]CellBasicDTO,
	cellAdvancedMap map[string][]CellAdvancedDTO,
	qosMap map[string][]QoSDTO,
	vonrMap map[string][]VoNRDTO,
	plmnForCellMap map[string][]PLMNForCellDTO,
	sliceForCellMap map[string][]SliceForCellDTO,
	floatForCellMap map[string][]FloatForCellDTO,
	sliceForPLMNMap map[string][]SliceForPlmnDTO,
	amfMap map[string][]AMFConfigDTO,
	vlanMap map[string][]VlanInterfaceConfigDTO,
	routeMap map[string][]RouteConfigDTO,
	ntpMap map[string]*NTPServerInfoDTO,
	identifierMap map[string]*BaseStationIdentifierDTO,
	clockMap map[string]*ClockDTO,
	ipSecMap map[string][]IPSecDTO,
	childSAMap map[string][]ChildSADTO,
) {
	cellBasicMap = map[string][]CellBasicDTO{}
	cellAdvancedMap = map[string][]CellAdvancedDTO{}
	qosMap = map[string][]QoSDTO{}
	vonrMap = map[string][]VoNRDTO{}
	plmnForCellMap = map[string][]PLMNForCellDTO{}
	sliceForCellMap = map[string][]SliceForCellDTO{}
	floatForCellMap = map[string][]FloatForCellDTO{}
	sliceForPLMNMap = map[string][]SliceForPlmnDTO{}
	amfMap = map[string][]AMFConfigDTO{}
	vlanMap = map[string][]VlanInterfaceConfigDTO{}
	routeMap = map[string][]RouteConfigDTO{}
	ntpMap = map[string]*NTPServerInfoDTO{}
	identifierMap = map[string]*BaseStationIdentifierDTO{}
	clockMap = map[string]*ClockDTO{}
	ipSecMap = map[string][]IPSecDTO{}
	childSAMap = map[string][]ChildSADTO{}

	for i := range d.cellBasic {
		v := d.cellBasic[i]
		cellBasicMap[v.GndId] = append(cellBasicMap[v.GndId], v)
	}
	for i := range d.cellAdvanced {
		v := d.cellAdvanced[i]
		cellAdvancedMap[v.GndId] = append(cellAdvancedMap[v.GndId], v)
	}
	for i := range d.qos {
		v := d.qos[i]
		qosMap[v.GndId] = append(qosMap[v.GndId], v)
	}
	for i := range d.vonr {
		v := d.vonr[i]
		vonrMap[v.GndId] = append(vonrMap[v.GndId], v)
	}
	for i := range d.plmnForCell {
		v := d.plmnForCell[i]
		plmnForCellMap[v.GndId] = append(plmnForCellMap[v.GndId], v)
	}
	for i := range d.sliceForCell {
		v := d.sliceForCell[i]
		sliceForCellMap[v.GndId] = append(sliceForCellMap[v.GndId], v)
	}
	for i := range d.floatForCell {
		v := d.floatForCell[i]
		floatForCellMap[v.GndId] = append(floatForCellMap[v.GndId], v)
	}
	for i := range d.sliceForPLMN {
		v := d.sliceForPLMN[i]
		sliceForPLMNMap[v.GndId] = append(sliceForPLMNMap[v.GndId], v)
	}
	for i := range d.amf {
		v := d.amf[i]
		amfMap[v.GndId] = append(amfMap[v.GndId], v)
	}
	for i := range d.vlan {
		v := d.vlan[i]
		vlanMap[v.GndId] = append(vlanMap[v.GndId], v)
	}
	for i := range d.route {
		v := d.route[i]
		routeMap[v.GndId] = append(routeMap[v.GndId], v)
	}
	for i := range d.ntp {
		v := d.ntp[i]
		ntpMap[v.GndId] = &v
	}
	for i := range d.baseId {
		v := d.baseId[i]
		identifierMap[v.GndId] = &v
	}
	for i := range d.clock {
		v := d.clock[i]
		clockMap[v.GndId] = &v
	}
	for i := range d.ipSec {
		v := d.ipSec[i]
		ipSecMap[v.GndId] = append(ipSecMap[v.GndId], v)
	}
	for i := range d.childSA {
		v := d.childSA[i]
		childSAMap[v.GndId] = append(childSAMap[v.GndId], v)
	}
	return
}

func buildInstanceNumbers(
	cellBasicMap map[string][]CellBasicDTO,
	cellAdvancedMap map[string][]CellAdvancedDTO,
	plmnForCellMap map[string][]PLMNForCellDTO,
) (cellIdMap, tacMap, plmnMap map[string]string) {
	cellIdMap = map[string]string{}
	tacMap = map[string]string{}
	plmnMap = map[string]string{}

	for gndId, list := range cellBasicMap {
		sorted := append([]CellBasicDTO{}, list...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].No < sorted[j].No })
		for i, v := range sorted {
			cellIdMap[cellKey(gndId, v.CellIdWithinGnb)] = strconv.Itoa(i + 1)
		}
	}
	for gndId, list := range cellAdvancedMap {
		byCell := map[string][]CellAdvancedDTO{}
		for _, v := range list {
			byCell[v.CellIdWithinGnb] = append(byCell[v.CellIdWithinGnb], v)
		}
		for cellId, group := range byCell {
			sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
			for i, v := range group {
				tacMap[tacKey(gndId, cellId, v.Tac)] = strconv.Itoa(i + 1)
			}
		}
	}
	for gndId, list := range plmnForCellMap {
		byCellTac := map[string][]PLMNForCellDTO{}
		for _, v := range list {
			key := cellKey(gndId, v.CellIdWithinGnb) + ":" + v.Tac
			byCellTac[key] = append(byCellTac[key], v)
		}
		for key, group := range byCellTac {
			sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
			gndId, cellId, tac := splitCellTacKey(key)
			for i, v := range group {
				plmnMap[plmnKey(gndId, cellId, tac, v.PLMN)] = strconv.Itoa(i + 1)
			}
		}
	}
	return
}

func splitCellTacKey(key string) (gndId, cellId, tac string) {
	// key = gndId:cellId:tac
	parts := strings.SplitN(key, ":", 3)
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

// ---------- generator ----------

type aosGenerator struct {
	gndId string

	cellBasicMap    map[string][]CellBasicDTO
	cellAdvancedMap map[string][]CellAdvancedDTO
	qosMap          map[string][]QoSDTO
	vonrMap         map[string][]VoNRDTO
	plmnForCellMap  map[string][]PLMNForCellDTO
	sliceForCellMap map[string][]SliceForCellDTO
	sliceForPLMNMap map[string][]SliceForPlmnDTO
	floatForCellMap map[string][]FloatForCellDTO
	amfMap          map[string][]AMFConfigDTO
	vlanMap         map[string][]VlanInterfaceConfigDTO
	routeMap        map[string][]RouteConfigDTO
	ntpMap          map[string]*NTPServerInfoDTO
	identifierMap   map[string]*BaseStationIdentifierDTO
	clockMap        map[string]*ClockDTO
	ipSecMap        map[string][]IPSecDTO
	childSAMap      map[string][]ChildSADTO

	cellIdMap map[string]string
	tacMap    map[string]string
	plmnMap   map[string]string

	params map[string]string
}

func (g *aosGenerator) add(name, value string) {
	if value == "" {
		return
	}
	g.params[name] = value
}

func (g *aosGenerator) run() {
	g.generateIdentifierParam()
	g.generateNTPParam()
	g.generateAMFParam()
	g.generateVlanParam()
	g.generateRouteParam()
	g.generateCellBasicParam()
	g.generateCellAdvancedParam()
	g.generateQosParam()
	g.generateVoNrParam()
	g.generatePlmnForCellParam()
	g.generateSliceForCellParam()
	g.generateSliceForPLMN()
	g.generateFloatForCellParam()
	g.generateIPSecParam()
	g.generateChildSaParam()
	g.generateClockParam()
}

func (g *aosGenerator) sortedCells() []CellBasicDTO {
	list := append([]CellBasicDTO{}, g.cellBasicMap[g.gndId]...)
	sort.Slice(list, func(i, j int) bool { return list[i].No < list[j].No })
	return list
}

func filterByCell[T any](list []T, cellId string, getCellId func(T) string) []T {
	var out []T
	for _, v := range list {
		if getCellId(v) == cellId {
			out = append(out, v)
		}
	}
	return out
}

func (g *aosGenerator) generateIdentifierParam() {
	d := g.identifierMap[g.gndId]
	if d == nil {
		return
	}
	p := "Device.Services.FAPService.1.FAPControl.NR.RAN.Common"
	g.add(p+".gNBId", d.GndId)
	g.add(p+".gNBName", d.GnbName)
	g.add(p+".gNBIdLength", d.GnbIdLength)
}

func (g *aosGenerator) generateNTPParam() {
	d := g.ntpMap[g.gndId]
	if d == nil {
		return
	}
	g.add("Device.DeviceInfo.MU.1.ClockSource", mapOrEmpty(clockSourceMap, d.ClockSource))
	g.add("Device.Time.Enable", enabledTo1(d.Enable))
	g.add("Device.Time.NTPServer1", d.NtpServer1)
	g.add("Device.Time.NTPServer2", d.NtpServer2)
	g.add("Device.Time.NTPServer3", d.NtpServer3)
	g.add("Device.Time.NTPServer4", d.NtpServer4)
	g.add("Device.Time.NTPServer5", d.NtpServer5)
	g.add("Device.Time.LocalTimeZone", d.LocalTimeZone)
}

func (g *aosGenerator) generateAMFParam() {
	for i, d := range g.amfMap[g.gndId] {
		idx := i + 1
		p := fmt.Sprintf("Device.Services.FAPService.1.FAPControl.NR.AMFPoolConfigParam.%d", idx)
		g.add(p+".AmfIP1", d.Ip1)
		g.add(p+".AmfIP2", d.Ip2)
		g.add(p+".LocalUpIPAddress", d.LocalUpIPAddress)
		g.add(p+".LocalUpIPV6Address", d.LocalUpIPV6Address)
		g.add(p+".LocalCpIPAddress", d.LocalCpIPAddress)
		g.add(p+".LocalCpIPV6Address", d.LocalCpIPV6Address)
		g.add(p+".PLMNID", d.PlmnId)
		g.add(p+".AmfPort", d.AmfPort)
		g.add(p+".AdminState", boolTrue(d.AdminState))
	}
}

func (g *aosGenerator) generateVlanParam() {
	for i, d := range g.vlanMap[g.gndId] {
		idx := i + 1
		p := fmt.Sprintf("Device.Ethernet.Interface.1.VlanInterface.%d", idx)
		g.add(p+".Id", d.Id)
		g.add(p+".Enable", enabledTo1(d.Enable))
		g.add(p+".IPv4Address.1.IPAddress", d.IPAddress)
		g.add(p+".IPv4Address.1.SubnetMask", d.SubnetMask)
		g.add(p+".IPv4Address.1.AddressingType", d.AddressingType)
		g.add(p+".IPv4Address.1.DefaultGateway", d.DefaultGateway)
		g.add(p+".IPv4Address.1.PortType", d.PortType)
		g.add(p+".IPv6Address.1.IPAddress", d.IPv6Address)
		g.add(p+".IPv6Address.1.PrefixLength", d.IpV6PrefixLength)
		g.add(p+".IPv6Address.1.Origin", d.IpV6Origin)
		g.add(p+".IPv6Address.1.DefaultGateway", d.IpV6DefaultGateway)
		g.add(p+".IPv6Address.1.PortType", d.IpV6PortType)
	}
}

func (g *aosGenerator) generateRouteParam() {
	for i, d := range g.routeMap[g.gndId] {
		idx := i + 1
		p := fmt.Sprintf("Device.Ethernet.IpRoute.%d", idx)
		g.add(p+".IpVer", ipv4ToNum(d.IpVersion))
		g.add(p+".DstIpNetwork", d.DstIpNetwork)
		g.add(p+".PrefixLength", d.PrefixLength)
		g.add(p+".GatewayIpAddress", d.GatewayIpAddress)
		g.add(p+".InterfaceName", d.InterfaceName)
	}
}

func (g *aosGenerator) generateCellBasicParam() {
	for _, cb := range g.sortedCells() {
		inst := g.cellIdMap[cellKey(cb.GndId, cb.CellIdWithinGnb)]
		if inst == "" {
			continue
		}
		p := "Device.Services.FAPService.1.CellConfig." + inst
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.IsPattern2Present", boolTrue(cb.Pattern2Present))
		g.add(p+".NR.RAN.Common.CellIdWithinGnb", cb.CellIdWithinGnb)
		g.add(p+".NR.RAN.RF.NRARFCNDL", cb.NRARFCNDL)
		g.add(p+".NR.RAN.RF.NRARFCNUL", cb.NRARFCNUL)
		g.add(p+".NR.RAN.RF.DLBandwidth", cb.DLBandwidth)
		g.add(p+".NR.RAN.RF.ULBandwidth", cb.ULBandwidth)
		g.add(p+".NR.RAN.RF.PhyCellID", cb.Pci)
		g.add(p+".NR.RAN.RF.SsbFrequency", cb.SsbFrequency)
		g.add(p+".NR.RAN.PHY.BWP.BWPUL.1.SrsConfig.Config.1.C_SRS", cb.CSRS1)
		g.add(p+".NR.RAN.PHY.BWP.BWPUL.1.SrsConfig.Config.2.C_SRS", cb.CSRS2)
		g.add(p+".NR.RAN.PHY.BWP.BWPDL.1.PDCCH.ControlResourceSetToAddModList.1.RbNumber", cb.RbNumber)
		g.add(p+".NR.RAN.PHY.RfConfig.FrequencyInfoDl.AbsoluteFrequencySSB", cb.AbsoluteFrequencySSB)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern1.DlULTransmissionPeriodicity", mapOrEmpty(patternPeriodicityMap, cb.Pattern1TransmissionPeriodicity))
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern1.NrofDownlinkSymbols", cb.Patter1NrofDownlinkSymbols)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern1.NrofUplinkSlots", cb.Patter1NrofUplinkSlots)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern1.NrofUplinkSymbols", cb.Patter1NrofUplinSymbols)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern1.NrofDownlinkSlots", cb.Patter1NrofDownlinkSlots)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern2.DlULTransmissionPeriodicity", mapOrEmpty(patternPeriodicityMap, cb.Pattern2TransmissionPeriodicity))
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern2.NrofDownlinkSymbols", cb.Patter2NrofDownlinkSymbols)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern2.NrofUplinkSlots", cb.Patter2NrofUplinkSlots)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern2.NrofUplinkSymbols", cb.Patter2NrofUplinSymbols)
		g.add(p+".NR.RAN.PHY.TddULDLConfigurationCommon.pattern2.NrofDownlinkSlots", cb.Patter2NrofDownlinkSlots)
		g.add(p+".NR.RAN.RF.FreqBandIndicator", cb.FreqBandIndicator)
		g.add(p+".NR.RAN.RouteIndexList", cb.RouteIndexList)
		g.add(p+".NR.RAN.RuList", cb.RuList)
		g.add(p+".NR.NGC.EnableSliceAccessControl", boolTrue(cb.EnableSliceAccessControl))
		g.add(p+".NR.NGC.Slice.SliceEnable", boolTrue(cb.SliceEnable))
		g.add(p+".NR.NGC.QosHeavyControl", boolTrue(cb.QosHeavyControl))
		g.add(p+".NR.NGC.Slice.SliceFreeAtIdleEnable", boolTrue(cb.SliceFreeAtIdleEnable))
	}
}

func (g *aosGenerator) generateCellAdvancedParam() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		group := filterByCell(g.cellAdvancedMap[g.gndId], cellId, func(d CellAdvancedDTO) string { return d.CellIdWithinGnb })
		sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
		for i, d := range group {
			idx := i + 1
			p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.CN.TA." + strconv.Itoa(idx)
			g.add(p+".Ranac", d.Ransc)
			g.add(p+".TAC", d.Tac)
		}
	}
}

func (g *aosGenerator) generateQosParam() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		for _, d := range filterByCell(g.qosMap[g.gndId], cellId, func(d QoSDTO) string { return d.CellIdWithinGnb }) {
			if d.Index == "" {
				continue
			}
			p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.NGC.QoS." + d.Index
			g.add(p+".FiveQI", d.FiveQI)
			g.add(p+".PLMNID", d.PLMNID)
			g.add(p+".RLC.Mode", d.RlcMode)
			g.add(p+".SNSSAI", d.SNSSAI)
		}
	}
}

func (g *aosGenerator) generateVoNrParam() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		group := filterByCell(g.vonrMap[g.gndId], cellId, func(d VoNRDTO) string { return d.CellIdWithinGnb })
		sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
		for i, d := range group {
			p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.VoNR.VoNRList." + strconv.Itoa(i+1)
			g.add(p+".Vo5qi", d.Vo5qi)
			g.add(p+".PLMNID", d.PlmnId)
			g.add(p+".VoSupport", d.VoSupport)
		}
	}
}

func (g *aosGenerator) generatePlmnForCellParam() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		for _, d := range filterByCell(g.plmnForCellMap[g.gndId], cellId, func(d PLMNForCellDTO) string { return d.CellIdWithinGnb }) {
			tacInst := g.tacMap[tacKey(cb.GndId, cellId, d.Tac)]
			plmnInst := g.plmnMap[plmnKey(cb.GndId, cellId, d.Tac, d.PLMN)]
			if tacInst == "" || plmnInst == "" {
				continue
			}
			p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.CN.TA." + tacInst + ".PLMNList." + plmnInst
			g.add(p+".PLMNID", d.PLMN)
		}
	}
}

func (g *aosGenerator) generateSliceForCellParam() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		for _, d := range filterByCell(g.sliceForCellMap[g.gndId], cellId, func(d SliceForCellDTO) string { return d.CellIdWithinGnb }) {
			if d.Index == "" {
				continue
			}
			p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.NGC.Slice.SliceList." + d.Index
			g.add(p+".SliceFreeAtIdle", boolTrue(d.SliceFreeAtIdle))
			g.add(p+".Heavy", d.Heavy)
		}
	}
}

func (g *aosGenerator) generateSliceForPLMN() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		group := filterByCell(g.sliceForPLMNMap[g.gndId], cellId, func(d SliceForPlmnDTO) string { return d.CellIdWithinGnb })
		sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
		// group by (tac, plmn)
		byKey := map[string][]SliceForPlmnDTO{}
		for _, v := range group {
			byKey[v.Tac+"|"+v.PLMN] = append(byKey[v.Tac+"|"+v.PLMN], v)
		}
		for key, members := range byKey {
			tac, plmn := splitTacPlmn(key)
			tacInst := g.tacMap[tacKey(cb.GndId, cellId, tac)]
			plmnInst := g.plmnMap[plmnKey(cb.GndId, cellId, tac, plmn)]
			if tacInst == "" || plmnInst == "" {
				continue
			}
			for i, d := range members {
				p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.CN.TA." + tacInst +
					".PLMNList." + plmnInst + ".SliceList." + strconv.Itoa(i+1)
				g.add(p+".SNSSAI", d.SNSSAI)
				g.add(p+".RrcCountWithinNssai", d.RrcCountWithinNssai)
			}
		}
	}
}

func splitTacPlmn(key string) (tac, plmn string) {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func (g *aosGenerator) generateFloatForCellParam() {
	for _, cb := range g.sortedCells() {
		cellId := cb.CellIdWithinGnb
		inst := g.cellIdMap[cellKey(cb.GndId, cellId)]
		if inst == "" {
			continue
		}
		group := filterByCell(g.floatForCellMap[g.gndId], cellId, func(d FloatForCellDTO) string { return d.CellIdWithinGnb })
		sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
		for i, d := range group {
			p := "Device.Services.FAPService.1.CellConfig." + inst + ".NR.NGC.Slice.FloatList." + strconv.Itoa(i+1)
			g.add(p+".Heavy", d.Heavy)
		}
	}
}

func (g *aosGenerator) generateIPSecParam() {
	for _, d := range g.ipSecMap[g.gndId] {
		if d.Index == "" {
			continue
		}
		p := "Device.IPsec.Conn." + d.Index
		g.add(p+".Enable", enableToBool(d.Enable))
		g.add(p+".AuthenticationMethod", d.AuthenticationMethod)
		g.add(p+".PreSharedKey", d.PreSharedKey)
		g.add(p+".Certs", d.Certs)
		g.add(p+".Gateway.SecGWServer1", d.SecGWServer1)
		g.add(p+".Gateway.SecGWServer2", d.SecGWServer2)
		g.add(p+".LocalId", d.LocalId)
		g.add(p+".RemoteId", d.RemoteId)
		g.add(p+".LocalPort", d.LocalPort)
		g.add(p+".LocalNattPort", d.LocalNattPort)
		g.add(p+".RemotePort", d.RemotePort)
		g.add(p+".LocalEapId", d.LocalEapId)
		g.add(p+".RemoteEapId", d.RemoteEapId)
		g.add(p+".OPC", d.Opc)
		g.add(p+".K", d.K)
		g.add(p+".EncryptionAlgorithms", d.EncryptionAlgorithms)
		g.add(p+".IntegrityAlgorithms", d.IntegrityAlgorithms)
		g.add(p+".DiffieHellmanGroupTransforms", d.DiffieHellmanGroupTransforms)
		g.add(p+".EnableVips", enableToBool(d.EnableVips))
		g.add(p+".EnableVipsV6", enableToBool(d.EnableVipsV6))
		g.add(p+".DpdDelay", d.DpdDelay)
		g.add(p+".Recovery", d.Recovery)
	}
}

func (g *aosGenerator) generateChildSaParam() {
	group := append([]ChildSADTO{}, g.childSAMap[g.gndId]...)
	sort.Slice(group, func(i, j int) bool { return group[i].No < group[j].No })
	for _, d := range group {
		if d.IpsecIndex == "" || d.ChildSAIndex == "" {
			continue
		}
		p := "Device.IPsec.Conn." + d.IpsecIndex + ".ChildSA." + d.ChildSAIndex
		g.add(p+".id", d.Id)
		g.add(p+".LocalTs", d.LocalTs)
		g.add(p+".RemoteTs", d.RemoteTs)
		g.add(p+".EncryptionAlgorithms", d.EncryptionAlgorithms)
		g.add(p+".IntegrityAlgorithms", d.IntegrityAlgorithms)
		g.add(p+".DiffieHellmanGroupTransforms", d.DiffieHellmanGroupTransforms)
	}
}

func (g *aosGenerator) generateClockParam() {
	d := g.clockMap[g.gndId]
	if d == nil {
		return
	}
	g.add("Device.DeviceInfo.MU.1.ClockSynMode", d.SyncMode)
}

// ---------- small utils ----------

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func parseInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return v, true
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func collectDeviceGroups(list []DeviceInfoDTO) []string {
	var out []string
	for _, d := range list {
		if d.DeviceGroup != "" {
			out = append(out, d.DeviceGroup)
		}
	}
	return out
}

// ---------- verifyData (§D) ----------

// verifyData performs the structural validations documented in §D and returns
// an *aosVerifyError (carrying the Java error code) on the first failure.
func (s *service) verifyData(d *nrAOSData) error {
	// Pre-compute sets.
	gndIdSet := map[string]bool{}
	for _, v := range d.deviceInfo {
		gndIdSet[v.GndId] = true
	}
	cellBasicSet := map[string]bool{} // gndId:cellId
	for _, v := range d.cellBasic {
		cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] = true
	}
	cellAdvancedTacSet := map[string]bool{} // gndId:cellId:tac
	for _, v := range d.cellAdvanced {
		cellAdvancedTacSet[tacKey(v.GndId, v.CellIdWithinGnb, v.Tac)] = true
	}
	plmnForCellSet := map[string]bool{} // gndId:cellId:tac:plmn
	for _, v := range d.plmnForCell {
		plmnForCellSet[plmnKey(v.GndId, v.CellIdWithinGnb, v.Tac, v.PLMN)] = true
	}

	// 1. device count > 1000
	if len(d.deviceInfo) > 1000 {
		return verr(10234, "device count exceeds 1000")
	}

	// 2. Device Info
	seenGnb := map[string]bool{}
	seenSerial := map[string]bool{}
	for _, v := range d.deviceInfo {
		if v.GndId == "" {
			return verr(10226, "Device Info: gNB ID is empty")
		}
		if v.SerialNumber == "" {
			return verr(10226, "Device Info: Serial Number is empty for gNB "+v.GndId)
		}
		if seenGnb[v.GndId] {
			return verr(10227, "Device Info: gNB ID duplicated: "+v.GndId)
		}
		seenGnb[v.GndId] = true
		if seenSerial[v.SerialNumber] {
			return verr(10227, "Device Info: Serial Number duplicated: "+v.SerialNumber)
		}
		seenSerial[v.SerialNumber] = true
	}
	// 10264: gnbId already allocated
	if err := s.checkGnbIdUsed(d.deviceInfo); err != nil {
		return err
	}

	// 3. Basic Identification
	seenBaseGnb := map[string]bool{}
	for _, v := range d.baseId {
		if v.GndId == "" {
			return verr(10226, "Basic Identification: gNB ID is empty")
		}
		if v.GnbIdLength == "" {
			return verr(10226, "Basic Identification: gNB ID Length is empty for "+v.GndId)
		}
		if !gndIdSet[v.GndId] {
			return verr(10207, "Basic Identification: gNB ID not in Device Info: "+v.GndId)
		}
		if seenBaseGnb[v.GndId] {
			return verr(10263, "Basic Identification: gNB ID duplicated: "+v.GndId)
		}
		seenBaseGnb[v.GndId] = true
	}

	// 4. NTP
	for _, v := range d.ntp {
		if !gndIdSet[v.GndId] {
			return verr(10208, "NTP: gndId not in Device Info: "+v.GndId)
		}
		if !localTimeZoneRe.MatchString(v.LocalTimeZone) {
			return verr(10228, "NTP: LocalTimeZone invalid for "+v.GndId)
		}
	}

	// 5. AMF
	for _, v := range d.amf {
		if !gndIdSet[v.GndId] {
			return verr(10209, "AMF: gndId not in Device Info: "+v.GndId)
		}
		if v.Ip1 == "" {
			return verr(10226, "AMF: IP1 is empty for "+v.GndId)
		}
		if v.PlmnId == "" {
			return verr(10226, "AMF: PLMN is empty for "+v.GndId)
		}
		if v.AdminState == "" {
			return verr(10226, "AMF: State is empty for "+v.GndId)
		}
		if !isIPv4(v.Ip1) {
			return verr(10229, "AMF: IP1 is not a valid IPv4 for "+v.GndId)
		}
		if v.Ip2 != "" && !isIPv4(v.Ip2) {
			return verr(10230, "AMF: IP2 is not a valid IPv4 for "+v.GndId)
		}
	}

	// 6. Network Interface (VLAN)
	for _, v := range d.vlan {
		if !gndIdSet[v.GndId] {
			return verr(10210, "Network Interface: gndId not in Device Info: "+v.GndId)
		}
		if v.Id == "" {
			return verr(10231, "Network Interface: vlan id is empty for "+v.GndId)
		}
		if v.Enable == "" {
			return verr(10232, "Network Interface: Enable is empty for "+v.GndId)
		}
		if v.AddressingType == "Static" {
			if v.IPAddress == "" || v.SubnetMask == "" || v.DefaultGateway == "" {
				return verr(10233, "Network Interface: Static addressing requires IP/Subnet/Gateway for "+v.GndId)
			}
		}
	}

	// 7. NR Cell Basic
	seenCell := map[string]bool{} // gndId:cellId
	for _, v := range d.cellBasic {
		if !gndIdSet[v.GndId] {
			return verr(10212, "NR Cell Basic: gndId not in Device Info: "+v.GndId)
		}
		if v.CellIdWithinGnb == "" {
			return verr(10213, "NR Cell Basic: Cell Id is empty for "+v.GndId)
		}
		if seenCell[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10214, "NR Cell Basic: Cell Id duplicated: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
		seenCell[cellKey(v.GndId, v.CellIdWithinGnb)] = true
		if v.NRARFCNUL == "" {
			return verr(10235, "NR Cell Basic: UL NRARFCN is empty")
		}
		if v.NRARFCNDL == "" {
			return verr(10236, "NR Cell Basic: DL NRARFCN is empty")
		}
		if v.DLBandwidth == "" {
			return verr(10237, "NR Cell Basic: DL Bandwidth is empty")
		}
		if v.ULBandwidth == "" {
			return verr(10238, "NR Cell Basic: UL Bandwidth is empty")
		}
		if v.Pci == "" {
			return verr(10239, "NR Cell Basic: PCI is empty")
		}
		if v.RbNumber == "" {
			return verr(10242, "NR Cell Basic: RB Number is empty")
		}
		if v.AbsoluteFrequencySSB == "" {
			return verr(10243, "NR Cell Basic: AbsoluteFrequencySSB is empty")
		}
		if v.FreqBandIndicator == "" {
			return verr(10244, "NR Cell Basic: FreqBandIndicator is empty")
		}
	}

	// 8. NR Cell Advanced
	seenTac := map[string]bool{} // gndId:cellId:tac
	for _, v := range d.cellAdvanced {
		if v.CellIdWithinGnb == "" {
			return verr(10215, "NR Cell Advanced: Cell Id is empty for "+v.GndId)
		}
		if !gndIdSet[v.GndId] {
			return verr(10216, "NR Cell Advanced: gndId not in Device Info: "+v.GndId)
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10217, "NR Cell Advanced: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
		if v.Tac == "" {
			return verr(10245, "NR Cell Advanced: TAC is empty")
		}
		if seenTac[tacKey(v.GndId, v.CellIdWithinGnb, v.Tac)] {
			return verr(10225, "NR Cell Advanced: TAC duplicated: "+tacKey(v.GndId, v.CellIdWithinGnb, v.Tac))
		}
		seenTac[tacKey(v.GndId, v.CellIdWithinGnb, v.Tac)] = true
	}

	// 9. QoS
	for _, v := range d.qos {
		if v.CellIdWithinGnb == "" {
			return verr(10218, "QoS: Cell Id is empty")
		}
		if !gndIdSet[v.GndId] {
			return verr(10219, "QoS: gndId not in Device Info: "+v.GndId)
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10220, "QoS: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
		if v.FiveQI == "" {
			return verr(10246, "QoS: 5QI is empty")
		}
		if v.PLMNID == "" {
			return verr(10247, "QoS: PLMN is empty")
		}
		if v.Index == "" {
			return verr(10247, "QoS: QoS Index is empty")
		}
	}

	// 10. VoNR
	for _, v := range d.vonr {
		if v.CellIdWithinGnb == "" {
			return verr(10221, "VoNR: Cell Id is empty")
		}
		if !gndIdSet[v.GndId] {
			return verr(10222, "VoNR: gndId not in Device Info: "+v.GndId)
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10223, "VoNR: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
	}

	// 11. PLMN For Cell
	for _, v := range d.plmnForCell {
		if v.CellIdWithinGnb == "" {
			return verr(10248, "PLMN For Cell: Cell Id is empty")
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10249, "PLMN For Cell: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
		if v.Tac == "" {
			return verr(10250, "PLMN For Cell: TAC is empty")
		}
		if !cellAdvancedTacSet[tacKey(v.GndId, v.CellIdWithinGnb, v.Tac)] {
			return verr(10251, "PLMN For Cell: TAC not in Cell Advanced: "+tacKey(v.GndId, v.CellIdWithinGnb, v.Tac))
		}
		if v.PLMN == "" {
			return verr(10252, "PLMN For Cell: PLMN is empty")
		}
	}

	// 12. Slice For Cell
	for _, v := range d.sliceForCell {
		if v.CellIdWithinGnb == "" {
			return verr(10253, "Slice For Cell: Cell Id is empty")
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10254, "Slice For Cell: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
	}

	// 13. Slice For PLMN
	for _, v := range d.sliceForPLMN {
		if v.CellIdWithinGnb == "" {
			return verr(10255, "Slice For PLMN: Cell Id is empty")
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10256, "Slice For PLMN: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
		if !cellAdvancedTacSet[tacKey(v.GndId, v.CellIdWithinGnb, v.Tac)] {
			return verr(10257, "Slice For PLMN: TAC not in Cell Advanced: "+tacKey(v.GndId, v.CellIdWithinGnb, v.Tac))
		}
		if !plmnForCellSet[plmnKey(v.GndId, v.CellIdWithinGnb, v.Tac, v.PLMN)] {
			return verr(10258, "Slice For PLMN: PLMN not in PLMN For Cell: "+plmnKey(v.GndId, v.CellIdWithinGnb, v.Tac, v.PLMN))
		}
	}

	// 14. Float For Cell
	for _, v := range d.floatForCell {
		if v.CellIdWithinGnb == "" {
			return verr(10260, "Float For Cell: Cell Id is empty")
		}
		if !cellBasicSet[cellKey(v.GndId, v.CellIdWithinGnb)] {
			return verr(10261, "Float For Cell: Cell Id not in Cell Basic: "+cellKey(v.GndId, v.CellIdWithinGnb))
		}
	}

	// 15. IP Sec
	for _, v := range d.ipSec {
		if !gndIdSet[v.GndId] {
			return verr(10266, "IP Sec: gndId not in Device Info: "+v.GndId)
		}
	}

	// 16. Child SA
	for _, v := range d.childSA {
		if !gndIdSet[v.GndId] {
			return verr(10267, "Child SA: gndId not in Device Info: "+v.GndId)
		}
		if v.IpsecIndex == "" {
			return verr(10310, "Child SA: Ipsec Index is empty")
		}
		if v.ChildSAIndex == "" {
			return verr(10311, "Child SA: Child SA Index is empty")
		}
	}

	// 17. Clock
	seenClockGnb := map[string]bool{}
	for _, v := range d.clock {
		if seenClockGnb[v.GndId] {
			return verr(10269, "Clock: gNB ID duplicated: "+v.GndId)
		}
		seenClockGnb[v.GndId] = true
		if !gndIdSet[v.GndId] {
			return verr(10270, "Clock: gndId not in Device Info: "+v.GndId)
		}
	}

	return nil
}

func (s *service) checkGnbIdUsed(deviceInfo []DeviceInfoDTO) error {
	type row struct {
		GnbId int
	}
	var rows []row
	if err := s.repo.DB().Table("ztp_gnbid_used").Select("gnb_id").Scan(&rows).Error; err != nil {
		// If the table is missing/unavailable, skip this check rather than
		// blocking the import.
		return nil
	}
	used := map[int]bool{}
	for _, r := range rows {
		used[r.GnbId] = true
	}
	for _, v := range deviceInfo {
		id, err := strconv.Atoi(v.GndId)
		if err != nil {
			continue
		}
		if used[id] {
			return verr(10264, "Device Info: gNB ID already in use: "+v.GndId)
		}
	}
	return nil
}
