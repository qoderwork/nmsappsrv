package restapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Service interface {
	ListDevices(c *gin.Context, offset, limit int) ([]RestDeviceVo, int64, error)
	GetDevice(c *gin.Context, id int64) (*RestDeviceVo, error)
	AddDevice(c *gin.Context, req *AddRestDeviceRequest) (*RestDeviceVo, error)
	ModifyDeviceById(c *gin.Context, id int64, req *ModifyRestDeviceRequest) error
	ModifyDeviceBySN(c *gin.Context, req *ModifyRestDeviceBySNRequest) error
	DeleteDevice(c *gin.Context, id int64) error
	GetDeviceParams(c *gin.Context, elementId int64) ([]RestParameterVo, error)
	SetDeviceParams(c *gin.Context, elementId int64, req *SetRestParameterRequest) error
	PresetDeviceParams(c *gin.Context, elementId int64, req *PresetParameterRequest) (*RequestStatusVo, error)
	GetRequestStatus(requestId string) (*RequestStatusVo, error)
	ListAlarms(c *gin.Context, offset, limit int) ([]RestAlarmVo, int64, error)
	SyncAlarms(c *gin.Context, req *SyncAlarmRequest) ([]RestAlarmVo, error)
	ClearAlarms(c *gin.Context, req *ClearAlarmRequest) error
	UploadUpgradeFile(c *gin.Context, fileName string, filePath string, fileSize int64) (*RestUpgradeFileVo, error)
	ListUpgradeFiles(c *gin.Context, offset, limit int) ([]RestUpgradeFileVo, int64, error)
	DeleteUpgradeFile(c *gin.Context, id int) error
	CreateUpgradeTask(c *gin.Context, req *RestUpgradeTaskRequest) (*RestUpgradeTaskVo, error)
	GetUpgradeTask(c *gin.Context, id int) (*RestUpgradeTaskVo, error)
	ListUpgradeTasks(c *gin.Context, offset, limit int) ([]RestUpgradeTaskVo, int64, error)
	ListTBGs(c *gin.Context, offset, limit int) ([]TBGVo, int64, error)
	GetTBGBySN(sn string) (*TBGVo, error)
	GetTBGByWanMac(mac string) (*TBGVo, error)
	AddTBGs(c *gin.Context, reqs []AddTBGRequest) ([]TBGVo, error)
	ModifyTBGs(c *gin.Context, reqs []ModifyTBGRequest) error
	DeleteTBGs(c *gin.Context, req *DeleteTBGRequest) error
	ListDeviceOnlineStatus(c *gin.Context) ([]DeviceOnlineStatusVo, error)
	GetDeviceOnlineStatus(c *gin.Context, elementId int64) (*DeviceOnlineStatusVo, error)
	GetACSSettings(c *gin.Context) (*RestACSConfigVo, error)
	UpdateACSSettings(c *gin.Context, req *RestUpdateACSConfigRequest) error
	SnmpGet(c *gin.Context, req *SnmpGetRequest) error
	SnmpSet(c *gin.Context, req *SnmpSetRequest) error
	ListSnmpOperationLogs(c *gin.Context, offset, limit int) ([]SnmpOperationLogVo, int64, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// newService builds a Service from an injected Repository (used by tests).
func newService(repo Repository) Service {
	return &service{repo: repo}
}

// ============================
// Helper functions
// ============================

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefIntPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func derefInt64Ptr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// generateLinkHeader generates RFC 5988 Link headers for offset-based pagination
func generateLinkHeader(baseUrl string, offset, limit, total int) string {
	var links []string

	// next
	if offset+limit < total {
		nextOffset := offset + limit
		links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"next\"", baseUrl, nextOffset, limit))
	}

	// prev
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"prev\"", baseUrl, prevOffset, limit))
	}

	// first
	links = append(links, fmt.Sprintf("<%s?offset=0&limit=%d>; rel=\"first\"", baseUrl, limit))

	// last
	lastOffset := 0
	if total > 0 {
		lastOffset = ((total - 1) / limit) * limit
	}
	links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"last\"", baseUrl, lastOffset, limit))

	return strings.Join(links, ", ")
}
