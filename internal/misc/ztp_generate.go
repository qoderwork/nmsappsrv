package misc

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nmsappsrv/pkg/logger"
)

// ---------- AOS config XML model (mirrors Java autoConfigFile) ----------

type aosConfig struct {
	XMLName xml.Name `xml:"config"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

type aosDataModelSpecific struct {
	XMLName xml.Name    `xml:"dataModelSpecific"`
	Version string      `xml:"version,attr"`
	Configs []aosConfig `xml:"config"`
}

type aosFile struct {
	XMLName           xml.Name             `xml:"autoConfigFile"`
	GenerateTime      string               `xml:"generateTime,attr"`
	NetworkType       string               `xml:"networkType,attr"`
	SerialNumber      string               `xml:"serialNumber,attr"`
	Vendor            string               `xml:"vendor,attr,omitempty"`
	DataModelSpecific aosDataModelSpecific `xml:"dataModelSpecific"`
	VendorSpecific    string               `xml:"vendorSpecific"`
}

// cpeElementAOSView is a read-only projection of cpe_element used to build the
// AOS file. It intentionally carries only the columns needed for generation.
type cpeElementAOSView struct {
	NeNeid        int64   `gorm:"column:ne_neid"`
	SerialNumber  *string `gorm:"column:serial_number"`
	Manufacturer  *string `gorm:"column:manufacturer"`
	Product       *string `gorm:"column:product"`
	DeviceType    *string `gorm:"column:device_type"`
	ZtpParameters *string `gorm:"column:ztp_parameters"`
	WifiOrGpsInfo *string `gorm:"column:wifi_or_gps_info"`
}

func (cpeElementAOSView) TableName() string { return "cpe_element" }

// locationMode is the subset of wifi_or_gps_info we need for the AOS filename.
type locationMode struct {
	Mode string `json:"mode"`
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// GenerateAOSFile builds the AOS auto-config XML for a device, writes it to the
// configured ZTP directory, and records cpe_element.aos_file_name so the device
// can pull it via /acs-file-server/ztpFile. This mirrors Java's
// GenerateZTPFileThread.generateAOSFile with the same document structure
// (autoConfigFile -> dataModelSpecific V1.1.1 -> config name/value pairs).
//
// It returns the generated file name. External system registration (BMC/LMF/
// GMLC/MSAG) and the embedded SFTP listener (port 10022) are intentionally out
// of scope here and handled as config-gated no-ops elsewhere, per the ztp
// backfill spec (only the core — config generation + state machine — is in
// scope; external calls are pluggable/skippable).
func (s *service) GenerateAOSFile(elementId int64, setReady bool) (string, error) {
	var dev cpeElementAOSView
	if err := s.repo.DB().Select(
		"ne_neid, serial_number, manufacturer, product, device_type, ztp_parameters, wifi_or_gps_info",
	).Where("ne_neid = ? AND deleted = ?", elementId, false).First(&dev).Error; err != nil {
		return "", fmt.Errorf("load device %d for AOS generation: %w", elementId, err)
	}
	if dev.SerialNumber == nil || *dev.SerialNumber == "" {
		return "", fmt.Errorf("device %d has no serial number; cannot name AOS file", elementId)
	}
	sn := *dev.SerialNumber

	// ztp_parameters holds the TR-069 parameter name->value map the device
	// should apply. This is exactly the config-pair set Java serialises into
	// the AOS file (dto.getData()).
	if dev.ZtpParameters == nil || strings.TrimSpace(*dev.ZtpParameters) == "" {
		return "", fmt.Errorf("device %d has no ztp_parameters; nothing to generate", elementId)
	}
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(*dev.ZtpParameters), &params); err != nil {
		return "", fmt.Errorf("device %d ztp_parameters invalid: %w", elementId, err)
	}
	if len(params) == 0 {
		return "", fmt.Errorf("device %d ztp_parameters empty", elementId)
	}

	// Vendor: prefer manufacturer, fall back to product / device_type.
	vendor := strOrEmpty(dev.Manufacturer)
	if vendor == "" {
		vendor = strOrEmpty(dev.Product)
	}
	if vendor == "" {
		vendor = strOrEmpty(dev.DeviceType)
	}

	// Location mode (GPS/WIFI/...) from wifi_or_gps_info; default GPS.
	mode := "GPS"
	if dev.WifiOrGpsInfo != nil && *dev.WifiOrGpsInfo != "" {
		var lm locationMode
		if err := json.Unmarshal([]byte(*dev.WifiOrGpsInfo), &lm); err == nil && lm.Mode != "" {
			mode = lm.Mode
		}
	}

	// Build config pairs, sorted for deterministic output.
	names := make([]string, 0, len(params))
	for k := range params {
		names = append(names, k)
	}
	sort.Strings(names)
	configs := make([]aosConfig, 0, len(names))
	for _, k := range names {
		configs = append(configs, aosConfig{Name: k, Value: fmt.Sprintf("%v", params[k])})
	}

	doc := aosFile{
		GenerateTime: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
		NetworkType:  "NR",
		SerialNumber: sn,
		Vendor:       vendor,
		DataModelSpecific: aosDataModelSpecific{
			Version: "V1.1.1",
			Configs: configs,
		},
	}

	xmlBytes, err := xml.MarshalIndent(doc, "", "    ")
	if err != nil {
		return "", fmt.Errorf("marshal AOS xml for device %d: %w", elementId, err)
	}
	content := []byte(xml.Header + string(xmlBytes) + "\n")

	// Resolve output directory (mirrors Java AOSManagementServiceImpl.path /
	// GenerateZTPFileThread "ztp" subdir under cmFilePath).
	dir := s.cfg.FileServer.ZtpDir
	if dir == "" {
		dir = filepath.Join(s.cfg.FileServer.Root, "ztp")
	}
	if dir == "" {
		dir = "./data/acs-file-server/ztp"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create ztp dir %s: %w", dir, err)
	}

	ts := time.Now().UTC().Format("20060102150405")
	// Filename convention mirrors Java: AOS_<mode>_<tenancy>_<serial>_<ts>.xml
	// (tenancy defaults to 0; cpe_element has no tenant_id column in Go).
	fileName := fmt.Sprintf("AOS_%s_0_%s_%s.xml", mode, sn, ts)
	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		return "", fmt.Errorf("write AOS file %s: %w", filePath, err)
	}

	// Record the generated file name on the device so resolveZtpFile serves it.
	// When setReady is true (normal ZTP path) we also flip read_to_ztp so the
	// device pulls the file; when false (manual NR AOS import) we leave
	// read_to_ztp untouched — mirroring Java's updateAOSFileNameAndReadyToZTP
	// with FALSE.
	updates := map[string]interface{}{"aos_file_name": fileName}
	if setReady {
		updates["read_to_ztp"] = true
	}
	if err := s.repo.DB().Table("cpe_element").
		Where("ne_neid = ?", elementId).
		Updates(updates).Error; err != nil {
		return "", fmt.Errorf("update cpe_element.aos_file_name for device %d: %w", elementId, err)
	}

	logger.Infof("ZTP AOS file generated: %s for device %d (vendor=%s, params=%d)", fileName, elementId, vendor, len(configs))
	return fileName, nil
}

// ScanAndGenerateAOSFiles is the ZTPTask-style trigger: it selects devices that
// are ready for ZTP (read_to_ztp = true) but have no AOS file yet
// (aos_file_name IS NULL) and generates the AOS file for each. It returns the
// number of files successfully generated. A failure for one device does not
// abort the scan — it is logged and the next device is attempted.
func (s *service) ScanAndGenerateAOSFiles() (int, error) {
	ids, err := s.repo.FindReadyForZTPAOS()
	if err != nil {
		return 0, fmt.Errorf("scan ready devices for AOS generation: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	generated := 0
	for _, id := range ids {
		if _, err := s.GenerateAOSFile(id, true); err != nil {
			logger.Warnf("ztp-aos-gen: device %d skipped: %v", id, err)
			continue
		}
		generated++
	}
	return generated, nil
}
