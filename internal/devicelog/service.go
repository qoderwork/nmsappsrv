package devicelog

import (
	"context"
	"encoding/json"
	"fmt"
	"nmsappsrv/internal/config"
	"nmsappsrv/internal/middleware"
	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service defines the business-logic contract for device log management.
type Service interface {
	AddLogCollectionTask(c *gin.Context, req *AddLogCollectionRequest) error
	ListLogCollectionResults(c *gin.Context, req *ListLogCollectionResultRequest) ([]LogCollectionResultVo, int64, error)
	DeleteAllLogFile(req *DeleteAllLogFileRequest) error
	DeleteLogFile(req *DeleteLogFileRequest) error
	GetLogFile(logId int64) (string, error)
	ListLogFiles(c *gin.Context, req *ListLogFileRequest) ([]LogFileVo, int64, error)
	EnablePeriodicUpload(c *gin.Context, req *EnablePeriodicUploadRequest) error
	DisablePeriodicUpload(c *gin.Context, req *DisablePeriodicUploadRequest) error
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{
		repo: NewRepository(db),
	}
}

// newService creates a Service backed by the given Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) AddLogCollectionTask(c *gin.Context, req *AddLogCollectionRequest) error {
	licenseId := middleware.GetLicenseId(c)
	now := time.Now()
	ctx := context.Background()

	for _, elementId := range req.ElementIds {
		// Generate a UUID (stripped of dashes) to correlate the device's file
		// upload with this NeLog record. Mirrors Java's
		// `UUID.randomUUID().toString().replace("-", "")` in addDeviceLogCollectionTask.
		requestId := strings.ReplaceAll(uuid.New().String(), "-", "")

		// Create NeLog record. Mirrors Java DeviceLogFileLog creation:
		//   status=0 (pending), activeLog=false, requestId, logTime=now.
		status := 0
		activeLog := false
		log := NeLog{
			ElementId:   &elementId,
			Status:      &status,
			LogTime:     &now,
			LicenseId:   &licenseId,
			RequestId:   &requestId,
			IsActiveLog: &activeLog,
		}

		if err := s.repo.Create(&log); err != nil {
			logger.Errorf("Create NeLog error: %v", err)
			continue
		}

		// Build TR-069 Upload DTO. Mirrors Java `OperationDTOGenerateUtil.getDeviceLog`:
		//   TR069UploadDTO{type=LOG_FILE, url=fileServerIp+"/api/acs-file-server/upload/log/"+requestId+"/",
		//                   username, password}
		// The device POSTs the collected log to this URL via the Upload RPC. The
		// unified dispatcher's Upload family (CollectLog/Upload/LOG/Backup/...) routes
		// to tr069.OperationSender.SendUpload.
		fileServerBase := "http://localhost"
		if config.Cfg != nil && config.Cfg.TR069.FileServerIp != "" {
			fileServerBase = config.Cfg.TR069.FileServerIp
		}
		fsUser, fsPass := config.GetFileServerCredentials()
		uploadURL := fmt.Sprintf("%s/api/acs-file-server/upload/log/%s/", fileServerBase, requestId)
		uploadDTO := tr069UploadDTO{
			Type:       "LOG_FILE",
			URL:        uploadURL,
			Username:   fsUser,
			Password:   fsPass,
			CommandKey: fmt.Sprintf("log_%d", log.Id),
		}
		uploadJSON, err := json.Marshal(uploadDTO)
		if err != nil {
			logger.Errorf("marshal upload DTO for element %d: %v", elementId, err)
			continue
		}
		msg := opmsg.Message{
			EventType:      "CollectLog",
			NeNeid:         elementId,
			Operation:      "CollectLog",
			OperationParam: string(uploadJSON),
			OperationUser:  "system",
			ProtocolType:   opmsg.ProtocolTR069,
			ExpiredAt:      time.Now().Add(5 * time.Minute).UnixMilli(),
		}
		msgBytes, err := msg.Marshal()
		if err != nil {
			logger.Errorf("marshal opmsg for element %d: %v", elementId, err)
			continue
		}
		if err := redis.LPush(ctx, mq.OperationQueue, string(msgBytes)); err != nil {
			logger.Errorf("Push %s error: %v", mq.OperationQueue, err)
		}
	}

	return nil
}

func (s *service) ListLogCollectionResults(c *gin.Context, req *ListLogCollectionResultRequest) ([]LogCollectionResultVo, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// Tenant isolation: every caller is scoped to its own license; filter the
	// joined ne_log rows by license_id so one tenant cannot read another's logs.
	licenseId := middleware.GetLicenseId(c)
	offset := (req.Page - 1) * req.PageSize
	results, total, err := s.repo.FindByFilter(&licenseId, req.ElementId, req.DeviceType, req.Status, offset, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (s *service) DeleteAllLogFile(req *DeleteAllLogFileRequest) error {
	// Fetch all log records first to get file paths for physical file deletion
	logs, err := s.repo.FindAllByElementId(req.ElementId)
	if err != nil {
		logger.Errorf("FindAllByElementId error: %v", err)
		return err
	}

	// Delete physical log files from disk
	for _, log := range logs {
		if log.FilePath != nil && *log.FilePath != "" {
			if err := os.Remove(*log.FilePath); err != nil {
				logger.Warnf("Failed to delete log file %s: %v", *log.FilePath, err)
			}
		}
	}

	// Delete all ne_log records for this elementId
	if err := s.repo.DeleteByElementId(req.ElementId); err != nil {
		logger.Errorf("DeleteByElementId error: %v", err)
		return err
	}

	return nil
}

func (s *service) DeleteLogFile(req *DeleteLogFileRequest) error {
	// Get log record first
	log, err := s.repo.FindByID(req.LogId)
	if err != nil {
		return err
	}

	// Delete record
	if err := s.repo.DeleteByID(req.LogId); err != nil {
		logger.Errorf("Delete NeLog error: %v", err)
		return err
	}

	// Delete actual file from disk if log.FilePath is set
	if log.FilePath != nil && *log.FilePath != "" {
		if err := os.Remove(*log.FilePath); err != nil {
			logger.Warnf("Failed to delete log file %s: %v", *log.FilePath, err)
		}
	}

	return nil
}

func (s *service) GetLogFile(logId int64) (string, error) {
	log, err := s.repo.FindByID(logId)
	if err != nil {
		return "", err
	}

	if log.FilePath == nil {
		return "", apperror.ErrNotFound.WithMessage("log file path is empty")
	}

	return *log.FilePath, nil
}

func (s *service) ListLogFiles(c *gin.Context, req *ListLogFileRequest) ([]LogFileVo, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	// Tenant isolation: scope the element's log files to the caller's license so
	// a user cannot read another tenant's files by passing a foreign elementId.
	licenseId := middleware.GetLicenseId(c)
	offset := (req.Page - 1) * req.PageSize
	logs, total, err := s.repo.FindByElementId(req.ElementId, &licenseId, offset, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]LogFileVo, 0, len(logs))
	for _, log := range logs {
		vo := LogFileVo{
			Id:       log.Id,
			FileName: *log.FileName,
			FileSize: *log.FileSize,
		}
		if log.CollectionTime != nil {
			vo.CollectionTime = log.CollectionTime.Format("2006-01-02 15:04:05")
		}
		result = append(result, vo)
	}

	return result, total, nil
}

func (s *service) EnablePeriodicUpload(c *gin.Context, req *EnablePeriodicUploadRequest) error {
	// Get device info to determine TR-069 path (Device vs InternetGatewayDevice root).
	rootNode, err := s.repo.FindDeviceRootNode(req.ElementId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	prefix := "Device"
	if rootNode != nil && *rootNode == "InternetGatewayDevice" {
		prefix = "InternetGatewayDevice"
	}

	// Mirror Java enableLogPeriodicUpload: enable + URL + interval + optional creds.
	// Java sets {rootNode}.LogMgmt.PeriodicUploadEnable="1",
	// {rootNode}.LogMgmt.URL=getFileServerIp()+"/api/acs-file-server/log/",
	// {rootNode}.LogMgmt.PeriodicUploadInterval=3600*period, and conditionally
	// {rootNode}.LogMgmt.Username/Password from the file-server credential holder.
	fileServerBase := "http://localhost"
	if config.Cfg != nil && config.Cfg.TR069.FileServerIp != "" {
		fileServerBase = config.Cfg.TR069.FileServerIp
	}
	fsUser, fsPass := config.GetFileServerCredentials()

	ctx := context.Background()
	params := []soap.ParameterValueStruct{
		{Name: prefix + ".LogMgmt.PeriodicUploadEnable", Value: "1", Type: "int"},
		{Name: prefix + ".LogMgmt.URL", Value: fileServerBase + "/api/acs-file-server/log/", Type: "string"},
		{Name: prefix + ".LogMgmt.PeriodicUploadInterval", Value: strconv.Itoa(req.Interval), Type: "int"},
	}
	if fsUser != "" && fsPass != "" {
		params = append(params,
			soap.ParameterValueStruct{Name: prefix + ".LogMgmt.Username", Value: fsUser, Type: "string"},
			soap.ParameterValueStruct{Name: prefix + ".LogMgmt.Password", Value: fsPass, Type: "string"},
		)
	}

	// Dispatch TR-069 SetParameterValues command (Java EventType.SET_PARAMETER_VALUES
	// → unified device-operation dispatcher routes to tr069.OperationSender.SendSetParameterValues).
	paramJSON, err := json.Marshal(params)
	if err != nil {
		return apperror.Wrap(err, "ENABLE_PERIODIC_UPLOAD_MARSHAL_FAILED", 500, "failed to marshal parameter payload")
	}
	msg := opmsg.Message{
		EventType:      "SetParameterValues",
		NeNeid:         req.ElementId,
		Operation:      "SetParameterValues",
		OperationParam: string(paramJSON),
		OperationUser:  "system",
		ProtocolType:   opmsg.ProtocolTR069,
		ExpiredAt:      time.Now().Add(5 * time.Minute).UnixMilli(),
	}
	msgBytes, err := msg.Marshal()
	if err != nil {
		return apperror.Wrap(err, "ENABLE_PERIODIC_UPLOAD_MSG_MARSHAL_FAILED", 500, "failed to marshal opmsg")
	}
	if err := redis.LPush(ctx, mq.OperationQueue, string(msgBytes)); err != nil {
		logger.Errorf("Push %s error: %v", mq.OperationQueue, err)
		return apperror.Wrap(err, "ENABLE_PERIODIC_UPLOAD_PUSH_FAILED", 500, "failed to enqueue operation")
	}

	return nil
}

func (s *service) DisablePeriodicUpload(c *gin.Context, req *DisablePeriodicUploadRequest) error {
	// Get device info to determine TR-069 path (Device vs InternetGatewayDevice root).
	rootNode, err := s.repo.FindDeviceRootNode(req.ElementId)
	if err != nil {
		return apperror.ErrDeviceNotFound
	}

	prefix := "Device"
	if rootNode != nil && *rootNode == "InternetGatewayDevice" {
		prefix = "InternetGatewayDevice"
	}

	// Dispatch TR-069 SetParameterValues command to disable (set LogMgmt.PeriodicUploadEnable=0).
	// Java disableLogPeriodUpload sets {rootNode}.LogMgmt.PeriodicUploadEnable="0".
	ctx := context.Background()
	params := []soap.ParameterValueStruct{
		{Name: prefix + ".LogMgmt.PeriodicUploadEnable", Value: "0", Type: "int"},
	}
	paramJSON, err := json.Marshal(params)
	if err != nil {
		return apperror.Wrap(err, "DISABLE_PERIODIC_UPLOAD_MARSHAL_FAILED", 500, "failed to marshal parameter payload")
	}
	msg := opmsg.Message{
		EventType:      "SetParameterValues",
		NeNeid:         req.ElementId,
		Operation:      "SetParameterValues",
		OperationParam: string(paramJSON),
		OperationUser:  "system",
		ProtocolType:   opmsg.ProtocolTR069,
		ExpiredAt:      time.Now().Add(5 * time.Minute).UnixMilli(),
	}
	msgBytes, err := msg.Marshal()
	if err != nil {
		return apperror.Wrap(err, "DISABLE_PERIODIC_UPLOAD_MSG_MARSHAL_FAILED", 500, "failed to marshal opmsg")
	}
	if err := redis.LPush(ctx, mq.OperationQueue, string(msgBytes)); err != nil {
		logger.Errorf("Push %s error: %v", mq.OperationQueue, err)
		return apperror.Wrap(err, "DISABLE_PERIODIC_UPLOAD_PUSH_FAILED", 500, "failed to enqueue operation")
	}

	return nil
}

// tr069UploadDTO mirrors the schema of `internal/operation.dispatcher's
// private tr069UploadDTO (which parses the JSON in opmsg.Message.OperationParam
// for the Upload family). Kept here as a local type so the devicelog producer
// can build the JSON the dispatcher expects without reaching into the
// dispatcher's private types. JSON keys are lowercase to match the Java
// `com.waveoss.common.dto.TR069UploadDTO` field names.
type tr069UploadDTO struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	CommandKey string `json:"commandKey,omitempty"`
}
