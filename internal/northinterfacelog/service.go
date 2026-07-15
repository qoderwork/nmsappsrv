package northinterfacelog

import (
	"time"
)

// LogVO is the query response row — mirrors Java ListNorthInterfaceLogVO
// (id, logName, username, operationTime, results, requestData, deviceName,
// serialNumber, info, tenancyName). tenancyName is left empty in Go v1 because
// there is no tenancy-name lookup service yet; the field is kept for shape parity.
type LogVO struct {
	ID            uint      `json:"id"`
	LogName       string    `json:"logName"`
	Username      string    `json:"username"`
	OperationTime time.Time `json:"operationTime"`
	Results       int       `json:"results"`
	RequestData   string    `json:"requestData"`
	DeviceName    string    `json:"deviceName"`
	SerialNumber  string    `json:"serialNumber"`
	Info          string    `json:"info"`
	TenancyName   string    `json:"tenancyName"`
}

// deviceView is a minimal read-only projection of cpe_element used to resolve
// deviceName / serialNumber for a log's elementId (Java does the same join).
type deviceView struct {
	NeNeid       int64   `gorm:"column:ne_neid"`
	DeviceName   *string `gorm:"column:device_name"`
	SerialNumber *string `gorm:"column:serial_number"`
}

func (deviceView) TableName() string { return "cpe_element" }

// Service is the northbound audit-log service.
type Service struct {
	repo *Repository
}

// NewService builds a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Save persists an audit record (used by the middleware and any explicit call).
func (s *Service) Save(log *NorthInterfaceLog) error {
	return s.repo.Save(log)
}

// List returns a page of audit logs as VOs, enriching deviceName/serialNumber
// via a single batched lookup on cpe_element.
func (s *Service) List(f ListFilter, page, pageSize int) ([]LogVO, int64, error) {
	logs, total, err := s.repo.List(f, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]int64, 0, len(logs))
	for _, l := range logs {
		if l.ElementID != 0 {
			ids = append(ids, l.ElementID)
		}
	}

	devMap := map[int64]deviceView{}
	if len(ids) > 0 {
		var views []deviceView
		if err := s.repo.db.Table("cpe_element").
			Select("ne_neid, device_name, serial_number").
			Where("ne_neid IN ?", ids).
			Find(&views).Error; err == nil {
			for _, v := range views {
				devMap[v.NeNeid] = v
			}
		}
	}

	vos := make([]LogVO, 0, len(logs))
	for _, l := range logs {
		vo := LogVO{
			ID:            l.ID,
			LogName:       l.LogName,
			Username:      l.User,
			OperationTime: l.OperationTime,
			Results:       l.Result,
			RequestData:   l.RequestData,
			Info:          l.Info,
			TenancyName:   "",
		}
		if d, ok := devMap[l.ElementID]; ok {
			if d.DeviceName != nil {
				vo.DeviceName = *d.DeviceName
			}
			if d.SerialNumber != nil {
				vo.SerialNumber = *d.SerialNumber
			}
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}
