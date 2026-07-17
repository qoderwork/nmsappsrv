package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/eventlog"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
)

// Dispatcher routes a single `OperationMessage` to the matching
// `tr069.OperationSender.Send*` primitive, mirroring the switch in
// Java `Tr069MessageBuilder.buildSoapThenSend`.
//
// The switch keys are the `EventType` constant **string values** from
// `com.waveoss.common.constants.EventType` — all PascalCase. See
// `internal/operation/dispatcher_test.go` for the full table.
type Dispatcher struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
}

// NewDispatcher wires the dispatcher. `opSender` may be nil during early
// bootstrap; callers must check `opSender != nil` before invoking `Dispatch`.
func NewDispatcher(db *gorm.DB, opSender *tr069.OperationSender) *Dispatcher {
	return &Dispatcher{db: db, opSender: opSender}
}

// tr069DownloadDTO mirrors Java `com.waveoss.common.dto.TR069DownloadDTO`.
// `Type` carries the `TR069DownloadType` enum (string form, e.g. "CA_FILE",
// "CBSD_CERT_FILE", "UPGRADE_FILE", "ZTP_FILE", "LICENSE", etc.).
type tr069DownloadDTO struct {
	Type           string `json:"type"`
	URL            string `json:"url"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	FileSize       int    `json:"fileSize"`
	TargetFileName string `json:"targetFileName"`
	DelaySeconds   int    `json:"delaySeconds"`
	CommandKey     string `json:"commandKey"`
}

// tr069UploadDTO mirrors Java `com.waveoss.common.dto.TR069UploadDTO`.
// `Type` carries the `TR069UploadType` enum (string form, e.g. "LOG_FILE",
// "Vendor Capture File").
type tr069UploadDTO struct {
	Type         string `json:"type"`
	URL          string `json:"url"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DelaySeconds int    `json:"delaySeconds"`
	CommandKey   string `json:"commandKey"`
}

// mapDownloadFileType maps the Java TR069DownloadType enum string to the
// TR-069 standard FileType string, mirroring Java Tr069MessageBuilder.downloadFile().
// If the type is already a standard TR-069 FileType string (e.g. "1 Firmware
// Upgrade Image" from the upgrade worker), it is returned as-is.
func mapDownloadFileType(javaType string) string {
	switch javaType {
	case "UPGRADE_FILE":
		return "1 Firmware Upgrade Image"
	case "RRU_UPGRADE_FILE":
		return "4 RRU Software Version File"
	case "CONFIG_FILE", "CBSD_CERT_FILE":
		return "3 Vendor Configuration File"
	case "M_NORMAL_FILE":
		return "M Normal File"
	case "CHECK_PROCESS_FILE", "BATCH_PROCESS_FILE":
		return "101 Script File"
	case "ZTP_FILE":
		return "103 Base Station Startup File"
	case "CA_FILE":
		return "4 Vendor Certificate File"
	case "LICENSE":
		return "102 License File"
	default:
		// Already a standard TR-069 FileType string or unknown — pass through.
		return javaType
	}
}

// mapUploadFileType maps the Java TR069UploadType enum string to the
// TR-069 standard FileType string, mirroring Java Tr069MessageBuilder.upload().
func mapUploadFileType(javaType string) string {
	switch javaType {
	case "CONFIG_FILE":
		return "3 Vendor Configuration File 1"
	case "LOG_FILE":
		return "2 Vendor Log File"
	default:
		return javaType
	}
}

// Dispatch routes the message to the matching `Send*` primitive. Returns the
// dispatch error (nil on success); the Worker logs and continues on error.
func (d *Dispatcher) Dispatch(ctx context.Context, msg *opmsg.Message) error {
	if d.opSender == nil {
		return fmt.Errorf("dispatcher: opSender not wired")
	}
	if msg == nil {
		return fmt.Errorf("dispatcher: nil message")
	}
	if msg.NeNeid <= 0 {
		return fmt.Errorf("dispatcher: missing neNeid (operation=%s)", msg.Operation)
	}

	// 1. Resolve neNeid → serial number (every Send* takes `sn`).
	sn, lookupErr := d.lookupSerialNumber(ctx, msg.NeNeid)
	if lookupErr != nil {
		d.markFault(ctx, msg, lookupErr)
		return lookupErr
	}

	// 2. operationId is the (optional) CommandTrackId as a string; the
	// `Send*` primitives also generate their own headerId internally.
	operationId := ""
	if msg.CommandTrackId > 0 {
		operationId = strconv.FormatInt(msg.CommandTrackId, 10)
	}

	// 3. Switch by Operation (PascalCase, Java EventType constant values).
	switch msg.Operation {

	// --- Parameter / object CRUD ---
	case "GetParameterNames":
		path, nextLevel := parseGetParameterNamesParam(msg.OperationParam)
		return d.opSender.SendGetParameterNames(sn, path, nextLevel, operationId)

	case "GetParameterValues", "GetParameterValuesForCRM":
		names, err := parseStringSliceParam(msg.OperationParam)
		if err != nil {
			d.markFault(ctx, msg, err)
			return err
		}
		return d.opSender.SendGetParameterValues(sn, names, operationId)

	case "SetParameterValues", "SetParameterValuesForCRM", "BarCell", "LockPCI", "ActiveVersion":
		params, err := parseParameterValueStructSlice(msg.OperationParam)
		if err != nil {
			d.markFault(ctx, msg, err)
			return err
		}
		return d.opSender.SendSetParameterValues(sn, params, "", operationId)

	case "GetParameterAttributes":
		names, err := parseStringSliceParam(msg.OperationParam)
		if err != nil {
			d.markFault(ctx, msg, err)
			return err
		}
		return d.opSender.SendGetParameterAttributes(sn, names, operationId)

	case "SetParameterAttributes":
		attrs, err := parseSetParameterAttributesStructSlice(msg.OperationParam)
		if err != nil {
			d.markFault(ctx, msg, err)
			return err
		}
		return d.opSender.SendSetParameterAttributes(sn, &soap.SetParameterAttributes{ParameterList: attrs}, operationId)

	case "AddObject":
		// OperationParam carries the object path (or JSON-encoded path).
		objName := strings.TrimSpace(stripJSONString(msg.OperationParam))
		return d.opSender.SendAddObject(sn, objName, operationId)

	case "DeleteObject":
		objName := strings.TrimSpace(stripJSONString(msg.OperationParam))
		return d.opSender.SendDeleteObject(sn, objName, operationId)

	// --- Reboot / factory-reset / soft-reboot ---
	case "Reboot":
		return d.opSender.SendReboot(sn, operationId, msg.OperationUser, msg.ExpiredAt)

	case "SoftReboot":
		return d.opSender.SendSoftReboot(sn, operationId, msg.OperationUser, msg.ExpiredAt)

	case "FactoryReset":
		return d.opSender.SendFactoryReset(sn, operationId)

	// Java maps SHUTDOWN to executeMml (the same handler as MML_EXECUTE).
	// Go's mml/worker is a separate queue; here we treat SHUTDOWN as a
	// plain Reboot (the device's behaviour is the same: it powers down).
	case "Shutdown":
		return d.opSender.SendReboot(sn, operationId, msg.OperationUser, msg.ExpiredAt)

	// --- Download family (Java's `downloadFile` handler) ---
	case "Download", "ZTP", "AutoProvisioning",
		"CA_TASK", "UpdateCertificate",
		"Restore", "SendCBSDCertFile", "UpdateLicense":
		dl, err := parseDownloadDTO(msg.OperationParam)
		if err != nil {
			d.markFault(ctx, msg, err)
			return err
		}
		return d.opSender.SendDownload(sn, &soap.Download{
			CommandKey:     dl.CommandKey,
			FileType:       mapDownloadFileType(dl.Type),
			URL:            dl.URL,
			Username:       dl.Username,
			Password:       dl.Password,
			FileSize:       dl.FileSize,
			TargetFileName: dl.TargetFileName,
			DelaySeconds:   dl.DelaySeconds,
		}, operationId)

	// --- Upload family (Java's `upload` handler, also `LOG`/`CollectLog`) ---
	case "Upload", "LOG", "CollectLog", "Backup", "BackupDaily":
		up, err := parseUploadDTO(msg.OperationParam)
		if err != nil {
			d.markFault(ctx, msg, err)
			return err
		}
		return d.opSender.SendUpload(sn, &soap.Upload{
			CommandKey: up.CommandKey,
			FileType:   mapUploadFileType(up.Type),
			URL:        up.URL,
			Username:   up.Username,
			Password:   up.Password,
		}, operationId)

	// --- Capture (Java's startCapture/stopCapture/getCapture) ---
	case "StartCapture", "StopCapture", "GetCapture":
		cap, err := parseCaptureDTO(msg.OperationParam)
		if err != nil {
			// Some callers publish a bare `{}` for StopCapture; fall back to
			// a minimal capture with just the switch.
			cap = &soap.Capture{CaptureSwitch: mapStopStartSwitch(msg.Operation)}
		}
		return d.opSender.SendCapture(sn, cap, operationId)

	// --- Cancellation ---
	case "CancelUpgrade":
		commandKey := strings.TrimSpace(stripJSONString(msg.OperationParam))
		return d.opSender.SendCancelFutureUpgrade(sn, commandKey, operationId)

	case "BatchUpgradeEUAndRU":
		// Java sends via executeMml-style path. Go treats as a soft-reboot
		// (the actual upgrade flow is owned by `internal/upgrade/worker`).
		return d.opSender.SendReboot(sn, operationId, msg.OperationUser, msg.ExpiredAt)

	// --- Operations the Go rewrite does not yet implement ---
	case "AlarmSynchronization",
		"ReloadParameters", "ReloadPartParameter", "ReloadPlatformParameter",
		"MmlExecute", "CoreNetworkOperation",
		"DEVICE_SOFTWARE_UPGRADE", "RRU_SOFTWARE_SEND",
		"addTask", "modifiedTask", "deletedTask", "pausedTask", "resumedTask",
		"reloadLicense":
		err := fmt.Errorf("dispatcher: operation %q not implemented in Go rewrite (Java-only or routed via separate queue)", msg.Operation)
		d.markFault(ctx, msg, err)
		return err

	default:
		err := fmt.Errorf("dispatcher: unknown operation %q (neNeid=%d, commandTrackId=%d)", msg.Operation, msg.NeNeid, msg.CommandTrackId)
		d.markFault(ctx, msg, err)
		return err
	}
}

// lookupSerialNumber resolves neNeid → cpe_element.serial_number. The Java
// side does this once in `dealCommand` before `buildSoapThenSend`; Go's
// `OperationSender.Send*` primitives take `sn`, so the dispatcher performs the
// lookup exactly once.
func (d *Dispatcher) lookupSerialNumber(ctx context.Context, neNeid int64) (string, error) {
	if d.db == nil {
		return "", fmt.Errorf("dispatcher: db not wired")
	}
	type row struct {
		SerialNumber *string `gorm:"column:serial_number"`
	}
	var r row
	err := d.db.WithContext(ctx).
		Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", neNeid, false).
		First(&r).Error
	if err != nil {
		return "", fmt.Errorf("device %d lookup: %w", neNeid, err)
	}
	if r.SerialNumber == nil || *r.SerialNumber == "" {
		return "", fmt.Errorf("device %d has empty serial_number", neNeid)
	}
	return *r.SerialNumber, nil
}

// markFault records the dispatch failure on the originating event_log row so
// the operator can see *why* an operation never reached the device. Mirrors
// Java's `Receiver.operationQueue` catch-block behaviour: log and continue.
func (d *Dispatcher) markFault(ctx context.Context, msg *opmsg.Message, cause error) {
	logger.Errorf("dispatcher: operation=%s neNeid=%d trackId=%d failed: %v",
		msg.Operation, msg.NeNeid, msg.CommandTrackId, cause)

	if d.db == nil || msg.CommandTrackId <= 0 {
		return
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status":                2, // 2 = failed/timeout in Go's event_log convention
		"command_response_time": &now,
		"fault_info":            cause.Error(),
	}
	if err := d.db.WithContext(ctx).
		Model(&eventlog.EventLog{}).
		Where("id = ?", msg.CommandTrackId).
		Updates(updates).Error; err != nil {
		logger.Errorf("dispatcher: failed to mark event_log %d as failed: %v", msg.CommandTrackId, err)
	}
}

// --- OperationParam parsers ---

func parseGetParameterNamesParam(s string) (path string, nextLevel bool) {
	// Java sends either a bare path string or a JSON `{"path":"...","nextLevel":true}`.
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if !strings.HasPrefix(s, "{") {
		return s, false
	}
	var p struct {
		Path      string `json:"path"`
		NextLevel bool   `json:"nextLevel"`
	}
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return s, false
	}
	return p.Path, p.NextLevel
}

func parseStringSliceParam(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	// Accept either a JSON array of strings or a newline-separated list.
	if strings.HasPrefix(s, "[") {
		var out []string
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			return nil, fmt.Errorf("parse string slice: %w", err)
		}
		return out, nil
	}
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

func parseParameterValueStructSlice(s string) ([]soap.ParameterValueStruct, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty SetParameterValues payload")
	}
	if !strings.HasPrefix(s, "[") {
		return nil, fmt.Errorf("SetParameterValues payload must be a JSON array, got: %q", s)
	}
	var out []soap.ParameterValueStruct
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("parse ParameterValueStruct slice: %w", err)
	}
	return out, nil
}

func parseSetParameterAttributesStructSlice(s string) ([]soap.SetParameterAttributesStruct, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty SetParameterAttributes payload")
	}
	if !strings.HasPrefix(s, "[") {
		return nil, fmt.Errorf("SetParameterAttributes payload must be a JSON array, got: %q", s)
	}
	var out []soap.SetParameterAttributesStruct
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("parse SetParameterAttributesStruct slice: %w", err)
	}
	return out, nil
}

func parseDownloadDTO(s string) (*tr069DownloadDTO, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty Download payload")
	}
	var d tr069DownloadDTO
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return nil, fmt.Errorf("parse TR069DownloadDTO: %w", err)
	}
	return &d, nil
}

func parseUploadDTO(s string) (*tr069UploadDTO, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty Upload payload")
	}
	var u tr069UploadDTO
	if err := json.Unmarshal([]byte(s), &u); err != nil {
		return nil, fmt.Errorf("parse TR069UploadDTO: %w", err)
	}
	return &u, nil
}

func parseCaptureDTO(s string) (*soap.Capture, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty Capture payload")
	}
	var c soap.Capture
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return nil, fmt.Errorf("parse Capture: %w", err)
	}
	return &c, nil
}

func mapStopStartSwitch(op string) string {
	switch op {
	case "StartCapture":
		return "1"
	case "StopCapture":
		return "0"
	default:
		return ""
	}
}

// stripJSONString unwraps a JSON-quoted string ("foo") back to foo; if the
// input isn't a JSON string it is returned verbatim.
func stripJSONString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var out string
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out
		}
	}
	return s
}
