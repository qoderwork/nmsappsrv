package ssh

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
)

// Repository defines the data-access contract for SSH management.
type Repository interface {
	FindLabelByName(tenantId int, name string) (*SSHLabel, error)
	FindLabelByID(id int) (*SSHLabel, error)
	CreateLabel(label *SSHLabel) error
	DeleteLabel(id int) error
	UpdateLabel(label *SSHLabel) error
	ListLabels(tenantId int) ([]SSHLabel, error)
	FindTimerByElementId(elementId int64) (*SSHAccessTimerTask, error)
	CreateTimer(task *SSHAccessTimerTask) error
	UpdateTimer(task *SSHAccessTimerTask) error
	ListTimers(page, pageSize int, elementId *int64) ([]SSHAccessTimerTask, int64, error)
	FindExpiredTimers() ([]SSHAccessTimerTask, error)
	FindElementInfo(elementId int64) (sn string, deviceName string, err error)
	FindElementIdsByGroup(groupIds []string) ([]int64, error)
	GetTenancyName(tenantId int) string
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// ---------- SSH Label ----------

func (r *repository) FindLabelByName(tenantId int, name string) (*SSHLabel, error) {
	var label SSHLabel
	err := r.db.Where("tenant_id = ? AND name = ?", tenantId, name).First(&label).Error
	if err != nil {
		return nil, err
	}
	return &label, nil
}

func (r *repository) FindLabelByID(id int) (*SSHLabel, error) {
	var label SSHLabel
	err := r.db.First(&label, id).Error
	if err != nil {
		return nil, err
	}
	return &label, nil
}

func (r *repository) CreateLabel(label *SSHLabel) error {
	return r.db.Create(label).Error
}

func (r *repository) DeleteLabel(id int) error {
	return r.db.Delete(&SSHLabel{}, id).Error
}

func (r *repository) UpdateLabel(label *SSHLabel) error {
	return r.db.Save(label).Error
}

func (r *repository) ListLabels(tenantId int) ([]SSHLabel, error) {
	var labels []SSHLabel
	err := r.db.Where("tenant_id = ?", tenantId).Find(&labels).Error
	return labels, err
}

// ---------- SSH Access Timer ----------

func (r *repository) FindTimerByElementId(elementId int64) (*SSHAccessTimerTask, error) {
	var task SSHAccessTimerTask
	err := r.db.Where("element_id = ?", elementId).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) CreateTimer(task *SSHAccessTimerTask) error {
	return r.db.Create(task).Error
}

func (r *repository) UpdateTimer(task *SSHAccessTimerTask) error {
	return r.db.Save(task).Error
}

func (r *repository) ListTimers(page, pageSize int, elementId *int64) ([]SSHAccessTimerTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	query := r.db.Model(&SSHAccessTimerTask{})
	if elementId != nil {
		query = query.Where("element_id = ?", *elementId)
	}

	var total int64
	query.Count(&total)

	var tasks []SSHAccessTimerTask
	err := query.Order("deadline DESC").
		Limit(pageSize).Offset((page - 1) * pageSize).
		Find(&tasks).Error
	return tasks, total, err
}

func (r *repository) FindExpiredTimers() ([]SSHAccessTimerTask, error) {
	var tasks []SSHAccessTimerTask
	now := time.Now()
	err := r.db.Where("ssh_status = ? AND deadline < ?", "1", now).Find(&tasks).Error
	return tasks, err
}

func (r *repository) FindElementInfo(elementId int64) (sn string, deviceName string, err error) {
	var row struct {
		SN         string `gorm:"column:serial_number"`
		DeviceName string `gorm:"column:device_name"`
	}
	err = r.db.Table("cpe_element").
		Select("serial_number, device_name").
		Where("ne_neid = ? AND deleted = 0", elementId).
		Scan(&row).Error
	return row.SN, row.DeviceName, err
}

func (r *repository) FindElementIdsByGroup(groupIds []string) ([]int64, error) {
	if len(groupIds) == 0 {
		return nil, nil
	}
	var ids []int64
	err := r.db.Table("device_group_element_rel").
		Select("DISTINCT element_id").
		Where("group_id IN ?", groupIds).
		Pluck("element_id", &ids).Error
	return ids, err
}

func (r *repository) GetTenancyName(tenantId int) string {
	var name string
	r.db.Table("tenancy").Where("id = ?", tenantId).Pluck("name", &name)
	return name
}

// ---------- Service ----------

// Service defines the business-logic contract for SSH operations.
type Service interface {
	AddLabel(req *AddSSHLabelRequest, tenantId int) error
	DeleteLabel(id int) error
	ListLabels(tenantId int) ([]SSHLabel, error)
	UpdateLabel(req *UpdateSSHLabelRequest, tenantId int) error
	SetAccessTimer(req *SSHAccessTimerRequest, tenantId int, username string) ([]string, error)
	ListAccessTimers(req *ListSSHAccessTimerRequest) ([]SSHAccessTimerVO, int64, error)
	StartExpiredChecker()
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a new SSH service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// ---------- SSH Label methods ----------

func (s *service) AddLabel(req *AddSSHLabelRequest, tenantId int) error {
	existing, _ := s.repo.FindLabelByName(tenantId, req.Name)
	if existing != nil {
		return fmt.Errorf("label name already exists")
	}
	label := &SSHLabel{
		Name:      &req.Name,
		Content:   &req.Content,
		TenantId: &tenantId,
	}
	return s.repo.CreateLabel(label)
}

func (s *service) DeleteLabel(id int) error {
	return s.repo.DeleteLabel(id)
}

func (s *service) ListLabels(tenantId int) ([]SSHLabel, error) {
	return s.repo.ListLabels(tenantId)
}

func (s *service) UpdateLabel(req *UpdateSSHLabelRequest, tenantId int) error {
	existing, _ := s.repo.FindLabelByName(tenantId, req.Name)
	if existing != nil && existing.Id != req.Id {
		return fmt.Errorf("label name already exists")
	}
	label, err := s.repo.FindLabelByID(req.Id)
	if err != nil {
		return fmt.Errorf("label not found")
	}
	label.Name = &req.Name
	label.Content = &req.Content
	return s.repo.UpdateLabel(label)
}

// ---------- SSH Access Timer methods ----------

func (s *service) SetAccessTimer(req *SSHAccessTimerRequest, tenantId int, username string) ([]string, error) {
	elementIds := req.ElementIds
	if req.DeviceGroupIds != nil && len(req.DeviceGroupIds) > 0 {
		groupIds, err := s.repo.FindElementIdsByGroup(req.DeviceGroupIds)
		if err != nil {
			return nil, fmt.Errorf("resolve device groups: %w", err)
		}
		elementIds = append(elementIds, groupIds...)
	}
	if len(elementIds) == 0 {
		return nil, fmt.Errorf("no devices selected")
	}

	if req.Deadline <= 0 {
		return nil, apperror.ErrInvalidInput.WithMessage("deadline must be a positive number of minutes")
	}

	deadline := time.Now().Add(time.Duration(req.Deadline) * time.Minute)
	tenancyName := s.repo.GetTenancyName(tenantId)
	now := time.Now()

	// operationIds are returned to the caller (and the frontend) so the
	// async TR-069 pushes can be polled for progress — mirrors Java's
	// Result<List<Long>> returned by setDeadlineToDevices.
	var operationIds []string

	for _, eid := range elementIds {
		sn, deviceName, err := s.repo.FindElementInfo(int64(eid))
		if err != nil {
			logger.Errorf("ssh_timer: find element %d: %v", eid, err)
			continue
		}

		existing, _ := s.repo.FindTimerByElementId(int64(eid))
		if existing != nil {
			existing.Deadline = &deadline
			existing.SshStatus = strPtr("0") // SSH_STATUS_DISENABLE, aligned with Java buildSshAccessTimerTask
			existing.LatestModifyTime = &now
			s.repo.UpdateTimer(existing)
		} else {
			task := &SSHAccessTimerTask{
				TenancyName:      &tenancyName,
				TenantId:        &tenantId,
				ElementId:        int64Ptr(int64(eid)),
				SshStatus:        strPtr("0"), // SSH_STATUS_DISENABLE, aligned with Java buildSshAccessTimerTask
				DeviceName:       &deviceName,
				SerialNumber:     &sn,
				Deadline:         &deadline,
				LatestModifyTime: &now,
			}
			s.repo.CreateTimer(task)
		}

		// Push TR-069 SetParameterValues to enable SSH through the shared
		// OperationSender. This both delivers the SPV to the device queue
		// (tr069:queue:<sn>, consumed by the ACS) and registers a track
		// record carrying the operationId — the literal "operation_queue"
		// LPush used previously was never consumed by the ACS, so devices
		// never received the enable/disable.
		operationId := fmt.Sprintf("ssh:%d:%d", eid, time.Now().UnixMilli())
		operationIds = append(operationIds, operationId)
		spv := []soap.ParameterValueStruct{
			{Name: "Device.SecurityManagement.SshEnable", Value: "true", Type: "xsd:boolean"},
		}
		if tr069.DefaultSender == nil {
			logger.Errorf("ssh_timer: tr069.DefaultSender not wired; cannot push enable SSH for %d", eid)
			continue
		}
		if err := tr069.DefaultSender.SendSetParameterValues(sn, spv, "", operationId); err != nil {
			logger.Errorf("ssh_timer: push enable SSH for %d: %v", eid, err)
		}
	}
	return operationIds, nil
}

func (s *service) ListAccessTimers(req *ListSSHAccessTimerRequest) ([]SSHAccessTimerVO, int64, error) {
	tasks, total, err := s.repo.ListTimers(req.Page, req.PageSize, req.ElementId)
	if err != nil {
		return nil, 0, err
	}

	vos := make([]SSHAccessTimerVO, len(tasks))
	for i, t := range tasks {
		vo := SSHAccessTimerVO{
			Id:          t.Id,
			SshStatus:   strVal(t.SshStatus),
			Deadline:    t.Deadline,
			TenancyName: strVal(t.TenancyName),
		}
		if t.ElementId != nil {
			vo.ElementId = *t.ElementId
		}
		if t.DeviceName != nil {
			vo.DeviceName = *t.DeviceName
		}
		if t.SerialNumber != nil {
			vo.SerialNumber = *t.SerialNumber
		}
		vos[i] = vo
	}
	return vos, total, nil
}

// StartExpiredChecker runs a background goroutine that checks every 2 minutes
// for expired SSH access timers and disables SSH on those devices.
func (s *service) StartExpiredChecker() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.processExpiredTasks()
		}
	}()
}

func (s *service) processExpiredTasks() {
	tasks, err := s.repo.FindExpiredTimers()
	if err != nil {
		logger.Errorf("ssh_timer: find expired: %v", err)
		return
	}
	now := time.Now()

	for _, t := range tasks {
		eid := int64(0)
		if t.ElementId != nil {
			eid = *t.ElementId
		}

		// Push TR-069 to disable SSH through the shared OperationSender so the
		// SPV actually reaches the device (the former "operation_queue" LPush
		// was never consumed by the ACS).
		sn, _, _ := s.repo.FindElementInfo(eid)
		operationId := fmt.Sprintf("ssh_disable:%d:%d", eid, time.Now().UnixMilli())
		spv := []soap.ParameterValueStruct{
			{Name: "Device.SecurityManagement.SshEnable", Value: "false", Type: "xsd:boolean"},
		}
		if tr069.DefaultSender != nil && sn != "" {
			if err := tr069.DefaultSender.SendSetParameterValues(sn, spv, "", operationId); err != nil {
				logger.Errorf("ssh_timer: push disable SSH for %d: %v", eid, err)
			}
		} else {
			logger.Errorf("ssh_timer: tr069.DefaultSender not wired or sn empty; cannot push disable SSH for %d", eid)
		}

		disabled := "0"
		t.SshStatus = &disabled
		t.LatestModifyTime = &now
		s.repo.UpdateTimer(&t)
	}
}

// ---------- helpers ----------

func strPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64 { return &i }
func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// newService creates a Service backed by the given Repository (test/mock helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
