package upgrade

import (
	"gorm.io/gorm"
)

// Service contains the business logic for upgrade management.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------------------------------------------------------------------------
// UpgradeFile
// ---------------------------------------------------------------------------

// ListUpgradeFiles returns a paginated list of upgrade files.
func (s *Service) ListUpgradeFiles(tenancyId int, page, pageSize int) ([]UpgradeFile, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUpgradeFiles(tenancyId, offset, pageSize)
}

// UploadUpgradeFile persists a new upgrade file record.
func (s *Service) UploadUpgradeFile(f *UpgradeFile) error {
	return s.repo.CreateUpgradeFile(f)
}

// DeleteUpgradeFile removes an upgrade file by ID.
func (s *Service) DeleteUpgradeFile(id int) error {
	return s.repo.DeleteUpgradeFile(id)
}

// ---------------------------------------------------------------------------
// UpgradeTask
// ---------------------------------------------------------------------------

// ListUpgradeTasks returns a paginated list of upgrade tasks.
func (s *Service) ListUpgradeTasks(tenancyId int, page, pageSize int) ([]UpgradeTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUpgradeTasks(tenancyId, offset, pageSize)
}

// GetUpgradeTask returns a single upgrade task by ID.
func (s *Service) GetUpgradeTask(id int) (*UpgradeTask, error) {
	return s.repo.FindUpgradeTaskByID(id)
}

// CreateUpgradeTask persists a new upgrade task.
func (s *Service) CreateUpgradeTask(t *UpgradeTask) error {
	return s.repo.CreateUpgradeTask(t)
}

// ---------------------------------------------------------------------------
// UpgradeLog
// ---------------------------------------------------------------------------

// ListUpgradeLogs returns a paginated list of upgrade logs for the given task.
func (s *Service) ListUpgradeLogs(taskId int, page, pageSize int) ([]UpgradeLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindUpgradeLogs(taskId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// RebootTask
// ---------------------------------------------------------------------------

// CreateRebootTask persists a new reboot task.
func (s *Service) CreateRebootTask(t *RebootTask) error {
	return s.repo.CreateRebootTask(t)
}

// ListRebootTasks returns a paginated list of reboot tasks.
func (s *Service) ListRebootTasks(tenancyId int, page, pageSize int) ([]RebootTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindRebootTasks(tenancyId, offset, pageSize)
}

// ---------------------------------------------------------------------------
// RollbackTask
// ---------------------------------------------------------------------------

// CreateRollbackTask persists a new rollback task.
func (s *Service) CreateRollbackTask(t *RollbackTask) error {
	return s.repo.CreateRollbackTask(t)
}

// ListRollbackTasks returns a paginated list of rollback tasks.
func (s *Service) ListRollbackTasks(tenancyId int, page, pageSize int) ([]RollbackTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindRollbackTasks(tenancyId, offset, pageSize)
}
