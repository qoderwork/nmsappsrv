package misc

import (
	"time"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for miscellaneous operations.
type Service interface {
	ListBatchConfigLogs(tenancyId int, page, pageSize int) ([]BatchConfigurationLog, int64, error)
	ListMRData(elementId int64, page, pageSize int) ([]MRData, int64, error)
	ListNorthReports(licenseId int) ([]NorthReport, error)
	CreateNorthReport(r *NorthReport) error
	UpdateNorthReport(r *NorthReport) error
	DeleteNorthReport(id int) error
	ListRadius(tenancyId int) ([]Radius, error)
	SaveRadius(r *Radius) error
	DeleteRadius(id int) error
	ListOperatorLogs(tenancyId int, page, pageSize int) ([]SystemOperatorLog, int64, error)
	ListUploadFiles(page, pageSize int) ([]UploadFile, int64, error)
	CreateUploadFile(f *UploadFile) error
	DeleteUploadFile(id string) error

	ListZTPLogs(elementId int64) ([]ZTPLog, error)
	GetZTPSetting() (*ZTPSetting, error)
	SaveZTPSetting(setting *ZTPSetting) error
	ListZTPResults(req *ListZTPResultsRequest) ([]ZTPResultVo, int64, error)
	ListZTPRetryLogs(elementId int64) ([]ZTPRetryLogVo, error)
	ListHistoryZTPFiles(elementId int64, page, pageSize int) ([]HistoryZTPFileVo, int64, error)
	SetZTPStatus(req *SetZTPStatusRequest) error
	BatchReZTP(req *BatchReZTPRequest) error
	DeleteZTPFiles(req *DeleteZTPFileRequest) error

	ListBackupRestoreTasks(tenancyId int, page, pageSize int) ([]BackupOrRestoreTask, int64, error)
	CreateBackupRestoreTask(t *BackupOrRestoreTask) error
	BatchAddObject(req *BatchAddObjectRequest, username string, tenancyId int) error
	ListBatchAddObjectTasks(tenancyId int, page, pageSize int) ([]BatchAddObjectTaskVo, int64, error)
	ListBatchAddObjectTaskDetail(taskId int) ([]BatchAddObjectTaskDetailVo, error)
	CreateBackupTask(req *BackupRestoreRequest, username string, tenancyId int) error
	CreateRestoreTask(req *BackupRestoreRequest, username string, tenancyId int) error
	StartBackupRestoreTask(taskId int, username string) error
	CancelBackupRestoreTask(taskId int) error
	ListBackupRestoreTasksVo(tenancyId int, page, pageSize int) ([]BackupRestoreTaskVo, int64, error)
	ListBackupRestoreTaskDetail(taskId int) ([]BackupRestoreTaskDetailVo, error)

	ListBaseStationBackupInfo(req *ListBaseStationBackupInfoRequest, tenancyId int) ([]BaseStationBackupInfoVo, int64, error)
	ImportConfigFile(elementId int64, fileName string, fileData []byte, tenancyId int) (*ImportConfigFileResult, error)
	ExportConfigFile(elementIds []int64, tenancyId int) (string, error)
	CreateBSBackupTask(req *AddBSBackupTaskRequest, username string, tenancyId int) error
	CreateBSRestoreTask(req *AddBSRestoreTaskRequest, username string, tenancyId int) error
	CancelTask(taskId int) error
	StartBSBackupRestoreTask(taskId int, username string) error
	ListBSBackupTasks(tenancyId int, page, pageSize int) ([]BackupRestoreTaskVo, int64, error)
	ListDeviceBackupResult(taskId int, page, pageSize int) ([]DeviceBackupResultVo, int64, error)
	GetConfigFilePath(logId string) (string, error)

	GetDeviceSerialNumber(elementId int64) (string, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// newService builds a Service from an injected Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ---------- helpers ----------

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
func ptrTime(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format(time.RFC3339)
}
func ptrTimePtr(p *time.Time) *time.Time { return p }

// GetDeviceSerialNumber returns the serial number for a device, used by the
// ZTP provisioning handler so it no longer needs a direct *gorm.DB handle.
func (s *service) GetDeviceSerialNumber(elementId int64) (string, error) {
	return s.repo.GetDeviceSerialNumber(elementId)
}
