package mml

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"nmsappsrv/internal/mq"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for MML operations.
type Service interface {
	ListMmlSets(tenantId int) ([]MmlSet, error)
	ListMmlCommands(setId int) ([]MmlCommand, error)
	GetMmlCommandParams(commandId int) ([]MmlCommandParam, error)
	ExecuteMml(elementId int64, command string, uid string, username string, params map[string]interface{}) (*MmlExecuteResult, error)
	ListMmlResults(elementId int64, page, pageSize int) ([]MmlExecuteResult, int64, error)
	GetMmlResult(id int) (*MmlExecuteResult, error)
	GetMMLResultByEventLogIds(eventLogIds []int64) ([]GetMMLResultByEventLogIdsVO, error)
	ImportMMLAndParameter(reader io.Reader, version string, tenantId int) error
	GetMmlVersions(tenantId int) ([]string, error)
	GetMmlCommandsByVersion(version string, tenantId int) (*MmlVersionInfoVO, error)
	GetMmlCommandTree(tenantId int) ([]MmlSetVo, error)
	DeleteMmlByVersion(version string, tenantId int) error
	UploadBatchProcessFile(fileName, filePath string, fileSize int64, username string, tenantId int) (*BatchProcessFile, error)
	ListBatchProcessFiles(tenantId int) ([]BatchProcessFile, error)
	SendBatchProcessFile(id int, tenantId int) (*BatchProcessFile, error)
	CheckBatchProcessFile(id int) (*BatchProcessFile, error)
	ListBatchProcessLogs(batchFileId int) ([]BatchProcessLog, error)
	ListBatchExecuteResults(batchFileId int) ([]MmlExecuteResult, error)
	GetBatchProcessFile(id int) (*BatchProcessFile, error)
	DeleteBatchProcessFile(id int) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ListMmlSets returns all MML sets for the given license.
func (s *service) ListMmlSets(tenantId int) ([]MmlSet, error) {
	return s.repo.FindMmlSets(tenantId)
}

// ListMmlCommands returns all commands in the given MML set.
func (s *service) ListMmlCommands(setId int) ([]MmlCommand, error) {
	return s.repo.FindMmlCommands(setId)
}

// GetMmlCommandParams returns all parameters for the given command.
func (s *service) GetMmlCommandParams(commandId int) ([]MmlCommandParam, error) {
	return s.repo.FindMmlCommandParams(commandId)
}

// ExecuteMml creates a pending execution result record (status=0) and
// enqueues the MML command to the Redis queue for async dispatch.
// It generates a CMDUID (correlation key, 对齐 Java UUID) and resolves the
// command category (type) used by the Device.mml.<type>.CMD downlink path.
func (s *service) ExecuteMml(elementId int64, command string, uid string, username string, params map[string]interface{}) (*MmlExecuteResult, error) {
	now := time.Now()

	// Generate the correlation key written to Device.mml.CMDUID and echoed back
	// by the device in the MMLREPORT Inform (对齐 Java CMDUID).
	cmdUid := uuid.NewString()

	// Resolve the MML command category for the downlink path Device.mml.<type>.CMD.
	cmdType := "MML"
	if cmd, err := s.repo.FindMmlCommandByCommand(command); err == nil && cmd != nil &&
		cmd.Type != nil && *cmd.Type != "" {
		cmdType = *cmd.Type
	}

	result := &MmlExecuteResult{
		ElementId:     &elementId,
		Command:       &command,
		Uid:           &cmdUid,
		User:          &username,
		Status:        0,
		OperationTime: &now,
		SendTime:      &now,
	}
	if err := s.repo.Create(result); err != nil {
		return nil, err
	}

	// Enqueue MML command to Redis queue for async processing by MML worker
	msg := MMLMessage{
		ElementId: elementId,
		Command:   command,
		Params:    params,
		ResultId:  result.Id,
		CmdUid:    cmdUid,
		CmdType:   cmdType,
	}
	if err := mq.Enqueue(context.Background(), mq.MMLQueue, msg); err != nil {
		logger.Errorf("failed to enqueue MML command to queue: %v", err)
		// Result record is already created with status=0; the worker will pick it up
		// once the queue message is retried or manually re-enqueued.
	}

	return result, nil
}

// ListMmlResults returns a paginated list of execution results for an element.
func (s *service) ListMmlResults(elementId int64, page, pageSize int) ([]MmlExecuteResult, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindMmlExecuteResults(elementId, offset, pageSize)
}

// GetMmlResult returns a single execution result by ID.
func (s *service) GetMmlResult(id int) (*MmlExecuteResult, error) {
	return s.repo.FindByID(id)
}

// GetMMLResultByEventLogIds returns MML execution results for the given eventLogIds,
// 对齐 Java MmlManagementServiceImpl.getMMLResultByEventLogIds:
//   - 仅 status==3 (delivered) 的结果；
//   - result 文本：hasFault→faultString，否则 result 为空时回填 "The MML is successfully executed"；
//   - 仅当设备存在 (cpe_element 命中 ne_neid) 才纳入返回（对齐 Java byId != null）。
func (s *service) GetMMLResultByEventLogIds(eventLogIds []int64) ([]GetMMLResultByEventLogIdsVO, error) {
	if len(eventLogIds) == 0 {
		return []GetMMLResultByEventLogIdsVO{}, nil
	}

	results, err := s.repo.FindMmlExecuteResultsByEventLogIds(eventLogIds)
	if err != nil {
		return nil, err
	}

	// Collect distinct element ids for a single device-lookup batch.
	idSet := make(map[int64]struct{})
	for _, r := range results {
		if r.ElementId != nil {
			idSet[*r.ElementId] = struct{}{}
		}
	}
	elementIds := make([]int64, 0, len(idSet))
	for id := range idSet {
		elementIds = append(elementIds, id)
	}
	deviceMap := make(map[int64]DeviceNameSerial)
	if len(elementIds) > 0 {
		devs, derr := s.repo.FindDeviceNameSerialByElementIds(elementIds)
		if derr != nil {
			logger.Errorf("GetMMLResultByEventLogIds device lookup error: %v", derr)
		} else {
			for _, d := range devs {
				deviceMap[d.NeNeid] = d
			}
		}
	}

	ans := make([]GetMMLResultByEventLogIdsVO, 0, len(results))
	for _, r := range results {
		if r.Status != 3 || r.ElementId == nil {
			continue
		}
		dev, ok := deviceMap[*r.ElementId]
		if !ok {
			// Java: only adds to answer when the device lookup succeeds.
			continue
		}
		vo := GetMMLResultByEventLogIdsVO{
			Mml:          mmlDerefStr(r.Command),
			DeviceName:   dev.DeviceName,
			SerialNumber: dev.SerialNumber,
		}
		if r.HasFault {
			vo.Result = mmlDerefStr(r.FaultString)
		} else if r.Result == nil || *r.Result == "" {
			vo.Result = "The MML is successfully executed"
		} else {
			vo.Result = *r.Result
		}
		ans = append(ans, vo)
	}
	return ans, nil
}
