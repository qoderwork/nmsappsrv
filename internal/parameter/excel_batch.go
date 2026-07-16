package parameter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/misc"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// ---------------------------------------------------------------------------
// Excel-driven batch parameter configuration (Java batchParameterConfiguration)
// ---------------------------------------------------------------------------

// excelDeviceParams is the intermediate representation after parsing Excel.
type excelDeviceParams struct {
	SerialNumber string
	Params       map[string]string // paramName -> value
}

// BatchParameterConfiguration parses an Excel workbook and fires
// SetParameterValues for each device found by serial number. This mirrors
// Java ConfigurationManagementServiceImpl.batchParameterConfiguration:
// sheets → headers (param names) → data rows (serial keys) → dispatch.
func (s *service) BatchParameterConfiguration(excelBytes []byte, username string, tenancyId int) error {
	if len(excelBytes) == 0 {
		return fmt.Errorf("excel file must not be empty")
	}

	// 1. Parse Excel → per-device param maps.
	devices, err := parseExcelForBatchConfig(excelBytes)
	if err != nil {
		return fmt.Errorf("parse excel: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("no valid device rows found in Excel")
	}

	// 2. Create batch_configuration_log (task).
	now := time.Now()
	taskName := fmt.Sprintf("BatchParameterConfig-%d", now.UnixMilli())
	deviceCount := len(devices)
	task := &misc.BatchConfigurationLog{
		Name:          &taskName,
		OperationTime: &now,
		TenancyId:     &tenancyId,
		User:          &username,
		DeviceCount:   &deviceCount,
	}
	if err := s.repo.CreateBatchConfigLog(task); err != nil {
		return fmt.Errorf("create batch config log: %w", err)
	}

	// 3. Per serial: find device, dispatch, create device log.
	expiredAt := now.Add(5 * time.Minute).UnixMilli()

	for _, dev := range devices {
		// Resolve ne_neid from serial_number.
		var elementId int64
		if err := s.repo.DB().Table("cpe_element").
			Select("ne_neid").
			Where("serial_number = ? AND deleted = ?", dev.SerialNumber, false).
			Scan(&elementId).Error; err != nil || elementId == 0 {
			logger.Warnf("batch param config: device with serial %s not found, skipping", dev.SerialNumber)
			continue
		}

		// Blacklist check.
		var blCount int64
		s.repo.DB().Raw(`
			SELECT COUNT(*) FROM element_black_list
			WHERE serial_number = (SELECT serial_number FROM cpe_element WHERE ne_neid = ?)
		`, elementId).Count(&blCount)
		if blCount > 0 {
			logger.Warnf("batch param config: device %d is blacklisted, skipping", elementId)
			continue
		}

		// Build operation param JSON (same format as BatchParameterConfigurationDirect).
		entries := make([]setParamEntry, 0, len(dev.Params))
		for k, v := range dev.Params {
			entries = append(entries, setParamEntry{ParamName: k, ParamValue: v})
		}
		opParamJSON, _ := json.Marshal(entries)

		// Create EventLog (status=1 = pending).
		eventLogId, err := s.repo.InsertEventLog("SetParameterValues", elementId, username, 1, string(opParamJSON))
		if err != nil {
			logger.Errorf("batch param config: create event_log for device %d: %v", elementId, err)
			continue
		}

		// Push to Redis operation_queue.
		msg := opmsg.Message{
			EventType:      "SetParameterValues", // Java EventType.SET_PARAMETER_VALUES
			NeNeid:         elementId,
			Operation:      "SetParameterValues",
			OperationParam: string(opParamJSON),
			OperationUser:  username,
			CommandTrackId: eventLogId,
			ExpiredAt:      expiredAt,
		}
		msgJSON, _ := msg.Marshal()
		if err := redis.LPush(context.Background(), mq.OperationQueue, string(msgJSON)); err != nil {
			logger.Errorf("batch param config: redis push for device %d: %v", elementId, err)
		}

		// Create batch_configuration_device_log.
		dataStr := string(opParamJSON)
		deviceLog := &misc.BatchConfigurationDeviceLog{
			TaskId:     &task.Id,
			ElementId:  &elementId,
			Data:       &dataStr,
			EventLogId: &eventLogId,
		}
		if err := s.repo.CreateBatchConfigDeviceLog(deviceLog); err != nil {
			logger.Errorf("batch param config: device log for device %d: %v", elementId, err)
		}
	}

	logger.Infof("batch param config: dispatched %d devices from Excel", len(devices))
	return nil
}

// parseExcelForBatchConfig reads an Excel workbook and returns per-device
// parameter maps keyed by serial number (column 0). Headers (row 0, columns
// 1..n) are the TR-069 parameter paths. Multi-sheet = multiple device groups;
// devices are merged by serial number across sheets (last sheet wins).
func parseExcelForBatchConfig(excelBytes []byte) ([]excelDeviceParams, error) {
	f, err := excelize.OpenReader(bytes.NewReader(excelBytes))
	if err != nil {
		return nil, fmt.Errorf("open excel: %w", err)
	}
	defer f.Close()

	merged := make(map[string]map[string]string)

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) < 2 {
			continue // need at least header + 1 data row
		}

		// Header row: col 0 = serial label (ignored), col 1..n = param names.
		headers := rows[0]

		for r := 1; r < len(rows); r++ {
			row := rows[r]
			if len(row) < 2 {
				continue
			}
			sn := row[0]
			if sn == "" {
				continue
			}

			if _, ok := merged[sn]; !ok {
				merged[sn] = make(map[string]string)
			}

			for c := 1; c < len(row) && c < len(headers); c++ {
				paramName := headers[c]
				if paramName == "" {
					continue
				}
				if row[c] != "" {
					merged[sn][paramName] = row[c]
				}
			}
		}
	}

	if len(merged) == 0 {
		return nil, fmt.Errorf("no data rows found")
	}

	result := make([]excelDeviceParams, 0, len(merged))
	for sn, params := range merged {
		result = append(result, excelDeviceParams{
			SerialNumber: sn,
			Params:       params,
		})
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

// BatchParameterConfiguration handles POST /batch-configuration
// (multipart/form-data with an Excel file). Mirrors Java
// ConfigurationManagementController.batchParameterConfiguration.
func (h *Handler) BatchParameterConfiguration(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "missing 'file' in multipart form")
		return
	}

	src, err := file.Open()
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "cannot read uploaded file")
		return
	}
	defer src.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(src); err != nil {
		utils.Error(c, http.StatusInternalServerError, "cannot read file content")
		return
	}

	username := middleware.GetUsername(c)
	var tenancyId int
	if v, ok := c.Get("tenancy_id"); ok {
		switch t := v.(type) {
		case string:
			tenancyId, _ = strconv.Atoi(t)
		case int:
			tenancyId = t
		}
	}
	if err := h.svc.BatchParameterConfiguration(buf.Bytes(), username, tenancyId); err != nil {
		utils.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.Success(c, nil)
}
