package mml

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"

	"gorm.io/gorm"
)

// MMLMessage is the JSON payload pushed to the queue:mml Redis queue.
type MMLMessage struct {
	ElementId int64                  `json:"element_id"`
	Command   string                 `json:"command"`
	Params    map[string]interface{} `json:"params"`
	ResultId  int                    `json:"result_id"`
	// CmdUid is the correlation key written to Device.mml.CMDUID on the device,
	// echoed back in the MMLREPORT Inform so the result can be matched (对齐 Java CMDUID).
	CmdUid string `json:"cmd_uid"`
	// CmdType is the MML command category (e.g. "MTN") used in the downlink path
	// Device.mml.<CmdType>.CMD (对齐 Java command.getType()).
	CmdType string `json:"cmd_type"`
}

// MMLWorker consumes messages from the MML queue and dispatches
// MML commands to devices via TR-069 SetParameterValues.
type MMLWorker struct {
	db       *gorm.DB
	opSender *tr069.OperationSender
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
}

// NewMMLWorker creates a new MMLWorker.
func NewMMLWorker(db *gorm.DB, msgManager *tr069.MessageManager) *MMLWorker {
	return &MMLWorker{
		db:       db,
		opSender: tr069.NewOperationSender(db, msgManager),
	}
}

// Start begins the MML command processing loop.
func (w *MMLWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	logger.Info("MML worker starting")

	utils.SafeGo("mml-worker", func() {
		w.pollLoop()
	})
}

// Stop stops the worker gracefully.
func (w *MMLWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
	logger.Info("MML worker stopped")
}

// IsRunning returns whether the worker is currently running.
func (w *MMLWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// pollLoop continuously polls the MML Redis queue for command messages.
func (w *MMLWorker) pollLoop() {
	for w.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		result, err := redis.BRPop(ctx, 5*time.Second, mq.MMLQueue)
		cancel()

		if err != nil {
			if err.Error() == "redis: nil" {
				continue
			}
			if !w.IsRunning() {
				return
			}
			logger.Debugf("MML worker queue poll error (may be timeout): %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		// result[0] is the queue name, result[1] is the message
		msgJSON := result[1]

		var msg MMLMessage
		if err := json.Unmarshal([]byte(msgJSON), &msg); err != nil {
			logger.Errorf("MML worker failed to unmarshal message: %v, data: %s", err, msgJSON)
			continue
		}

		logger.Infof("MML worker: processing command=%s for element=%d resultId=%d", msg.Command, msg.ElementId, msg.ResultId)
		w.processMmlCommand(&msg)
	}
}

// processMmlCommand handles a single MML command message.
// It looks up the device and sends the MML command to the device over TR-069
// via the Device.mml.* downlink channel (对齐 Java singleExecuteMML):
//   - Device.mml.CMDUID = correlation UUID
//   - Device.mml.<type>.CMD = MML command text (built by buildMML)
func (w *MMLWorker) processMmlCommand(msg *MMLMessage) {
	// 1. Look up the device serial number from element_id
	var sn string
	type deviceLookup struct {
		SerialNumber *string `gorm:"column:serial_number"`
	}
	var dev deviceLookup
	if err := w.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = ?", msg.ElementId, false).
		First(&dev).Error; err != nil {
		faultMsg := fmt.Sprintf("device %d not found: %v", msg.ElementId, err)
		logger.Errorf("MML worker: %s", faultMsg)
		UpdateResultStatusOnAck(w.db, msg.ResultId, false, faultMsg)
		return
	}

	if dev.SerialNumber == nil || *dev.SerialNumber == "" {
		faultMsg := fmt.Sprintf("device %d has no serial number", msg.ElementId)
		logger.Errorf("MML worker: %s", faultMsg)
		UpdateResultStatusOnAck(w.db, msg.ResultId, false, faultMsg)
		return
	}
	sn = *dev.SerialNumber

	// 2. Build the MML command text and the two TR-069 params written in a single SPV.
	cmdType := msg.CmdType
	if cmdType == "" {
		cmdType = "MML"
	}
	mmlText := buildMML(msg.Command, msg.Params)
	cmdUid := msg.CmdUid
	if cmdUid == "" {
		cmdUid = fmt.Sprintf("mml-%d", msg.ResultId)
	}

	spvParams := []soap.ParameterValueStruct{
		{Name: "Device.mml.CMDUID", Value: cmdUid, Type: "xsd:string"},
		{Name: fmt.Sprintf("Device.mml.%s.CMD", cmdType), Value: mmlText, Type: "xsd:string"},
	}

	// 3. Send SetParameterValues to the device. The operationId carries the result
	//    id (mml:<resultId>) so processSetParameterValuesResponse can mark the
	//    result delivered (status=3) when the device ACKs (对齐 Java MmlMessageProcessor).
	operationId := fmt.Sprintf("mml:%d", msg.ResultId)
	paramKey := fmt.Sprintf("mml_%d", msg.ResultId)

	if err := w.opSender.SendSetParameterValues(sn, spvParams, paramKey, operationId); err != nil {
		faultMsg := fmt.Sprintf("SPV send failed for device %d (SN=%s): %v", msg.ElementId, sn, err)
		logger.Errorf("MML worker: %s", faultMsg)
		UpdateResultStatusOnAck(w.db, msg.ResultId, false, faultMsg)
		return
	}

	// 4. Mark status=2 (dispatched to device). status=3 (delivered) is set by the
	//    SPV-response handler once the device ACKs the SetParameterValues.
	w.updateResultStatus(msg.ResultId, 2, "")
	logger.Infof("MML worker: command=%s sent to device %d (SN=%s), operationId=%s, cmdUid=%s",
		msg.Command, msg.ElementId, sn, operationId, cmdUid)
}

// updateResultStatus updates the MmlExecuteResult status in the database.
// status: 0=created, 2=dispatched, 3=delivered(ACK). Failures are encoded via has_fault.
func (w *MMLWorker) updateResultStatus(resultId int, status int, faultString string) {
	updates := map[string]interface{}{
		"status": status,
	}
	if faultString != "" {
		updates["fault_string"] = faultString
		updates["has_fault"] = true
	}
	if err := w.db.Model(&MmlExecuteResult{}).Where("id = ?", resultId).Updates(updates).Error; err != nil {
		logger.Errorf("MML worker: failed to update result %d status: %v", resultId, err)
	}
}

// UpdateResultStatusOnAck is invoked by the TR-069 SPV-response handler when the
// device ACKs (or faults on) the SetParameterValues that carried an MML command.
// 对齐 Java: status=3 means "delivered to device"; success vs failure is encoded
// in has_fault (a failed ACK still lands in status=3 with has_fault=true).
func UpdateResultStatusOnAck(db *gorm.DB, resultId int, success bool, faultString string) {
	updates := map[string]interface{}{"status": 3}
	if !success {
		updates["has_fault"] = true
		if faultString != "" {
			updates["fault_string"] = faultString
		}
	}
	if err := db.Model(&MmlExecuteResult{}).Where("id = ?", resultId).Updates(updates).Error; err != nil {
		logger.Errorf("mml: failed to update result %d status on ACK: %v", resultId, err)
	}
}

// buildMML assembles the MML command text from a command name and its parameters,
// 对齐 Java MmlManagementServiceImpl.buildMML:
//
//	<CMD>:<NAME1>=<VAL1>,<NAME2>=<VAL2>;
//
// Values are NOT quoted; keys are sorted for deterministic output.
func buildMML(command string, params map[string]interface{}) string {
	if len(params) == 0 {
		return command + ";"
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(command)
	sb.WriteString(":")
	for i, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(paramValueToString(params[k]))
		if i != len(keys)-1 {
			sb.WriteString(",")
		}
	}
	sb.WriteString(";")
	return sb.String()
}

// paramValueToString renders an MML parameter value (values are unquoted in MML).
func paramValueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}
