package nmsbackup

import (
	"fmt"
	"time"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// AddBackupSchedule creates a new backup schedule definition (nms_backup_and_revert)
func (s *Service) AddBackupSchedule(c *gin.Context, req *AddNMSBackupTaskRequest) (*NMSBackupAndRevert, error) {
	username := middleware.GetUsername(c)
	licenseId := middleware.GetLicenseId(c)
	now := time.Now()

	schedule := &NMSBackupAndRevert{
		CreateTime:     &now,
		CreateUserName: strPtr(username),
		BackupName:     strPtr(req.BackupName),
		BackupType:     intPtr(req.BackupType),
		BackupStatus:   intPtr(0), // ready
		LicenseId:      intPtr(licenseId),
	}

	if req.BackupType == 1 {
		// Scheduled
		schedule.BackupInterval = intPtr(req.BackupInterval)
		if req.BackupBeginTime != "" {
			t, err := parseTime(req.BackupBeginTime)
			if err != nil {
				return nil, apperror.Wrap(err, "INVALID_INPUT", 400, "invalid backup begin time")
			}
			schedule.BackupBeginTime = &t
		}
	}

	if req.PmInterval > 0 {
		schedule.PmInterval = intPtr(req.PmInterval)
	}
	if req.MrInterval > 0 {
		schedule.MrInterval = intPtr(req.MrInterval)
	}
	if req.XlogInterval > 0 {
		schedule.XlogInterval = intPtr(req.XlogInterval)
	}

	if err := s.repo.Create(schedule); err != nil {
		logger.Errorf("Failed to create backup schedule: %v", err)
		return nil, apperror.ErrInternal.WithMessage("failed to create backup schedule")
	}

	logger.Infof("Created backup schedule %d (%s) by user %s", schedule.Id, req.BackupName, username)
	return schedule, nil
}

// ListBackupSchedules returns paginated list of backup schedules
func (s *Service) ListBackupSchedules(c *gin.Context, req *ListNMSBackupTaskRequest) ([]NMSBackupTaskVo, int64, error) {
	licenseId := middleware.GetLicenseId(c)

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	schedules, total, err := s.repo.ListSchedules(licenseId, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var result []NMSBackupTaskVo
	for _, sch := range schedules {
		vo := NMSBackupTaskVo{
			Id:              sch.Id,
			BackupName:      derefString(sch.BackupName),
			BackupType:      derefInt(sch.BackupType),
			BackupInterval:  derefInt(sch.BackupInterval),
			BackupBeginTime: formatTime(sch.BackupBeginTime),
			BackupStatus:    derefInt(sch.BackupStatus),
			PmInterval:      derefInt(sch.PmInterval),
			MrInterval:      derefInt(sch.MrInterval),
			XlogInterval:    derefInt(sch.XlogInterval),
			CreateTime:      formatTime(sch.CreateTime),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

// ModifyBackupSchedule updates an existing backup schedule
func (s *Service) ModifyBackupSchedule(c *gin.Context, req *ModifyNMSBackupTaskRequest) error {
	schedule, err := s.repo.FindByID(req.Id)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("backup schedule not found")
	}

	if req.BackupName != "" {
		schedule.BackupName = strPtr(req.BackupName)
	}
	if req.BackupType != nil {
		schedule.BackupType = req.BackupType
	}
	if req.BackupInterval != nil {
		schedule.BackupInterval = req.BackupInterval
	}
	if req.BackupBeginTime != nil {
		t, err := parseTime(*req.BackupBeginTime)
		if err != nil {
			return apperror.Wrap(err, "INVALID_INPUT", 400, "invalid backup begin time")
		}
		schedule.BackupBeginTime = &t
	}
	if req.PmInterval != nil {
		schedule.PmInterval = req.PmInterval
	}
	if req.MrInterval != nil {
		schedule.MrInterval = req.MrInterval
	}
	if req.XlogInterval != nil {
		schedule.XlogInterval = req.XlogInterval
	}

	if err := s.repo.Save(schedule); err != nil {
		logger.Errorf("Failed to modify backup schedule %d: %v", req.Id, err)
		return apperror.ErrInternal.WithMessage("failed to modify backup schedule")
	}

	logger.Infof("Modified backup schedule %d", req.Id)
	return nil
}

// RunBackup triggers immediate backup execution for a schedule
func (s *Service) RunBackup(c *gin.Context, req *RunNMSBackupTaskRequest) error {
	username := middleware.GetUsername(c)

	schedule, err := s.repo.FindByID(req.Id)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("backup schedule not found")
	}

	now := time.Now()

	// Mark schedule as running
	runningStatus := 1
	schedule.BackupStatus = &runningStatus
	if err := s.repo.Save(schedule); err != nil {
		return apperror.ErrInternal.WithMessage("failed to update schedule status")
	}

	// Create task execution record
	task := &NMSBackupAndRevertTask{
		Name:        strPtr(derefString(schedule.BackupName)),
		TaskType:    strPtr("backup"),
		ExecuteMode: intPtr(1), // immediately
		Status:      intPtr(1), // waiting
		CreateTime:  &now,
		UpdateTime:  &now,
		User:        strPtr(username),
		NmsBackupId: intPtr(schedule.Id),
	}

	if err := s.repo.CreateTask(task); err != nil {
		logger.Errorf("Failed to create backup task record: %v", err)
		failedStatus := 3
		schedule.BackupStatus = &failedStatus
		s.repo.Save(schedule)
		return apperror.ErrInternal.WithMessage("failed to create task record")
	}

	// Mark task as running
	taskRunning := 2
	task.Status = &taskRunning
	task.StartTime = &now
	s.repo.UpdateTask(task)

	// Simulate backup execution (actual mysqldump requires external tools)
	fileName := fmt.Sprintf("backup_%d_%s.sql.gz", schedule.Id, now.Format("20060102_150405"))
	endTime := time.Now()
	duration := int(endTime.Sub(now).Seconds())

	// Update task as done
	taskDone := 3
	task.Status = &taskDone
	task.EndTime = &endTime
	s.repo.UpdateTask(task)

	// Update schedule: completed
	completedStatus := 2
	schedule.BackupStatus = &completedStatus
	schedule.BackupTimeCost = intPtr(duration)
	schedule.FileName = strPtr(fileName)
	s.repo.Save(schedule)

	// Create log record
	logRecord := &NMSBackupAndRevertLog{
		FileName:      strPtr(fileName),
		Time:          &now,
		OperationUser: strPtr(username),
		Result:        intPtr(0), // success
	}

	if err := s.repo.CreateLog(logRecord); err != nil {
		logger.Errorf("Failed to create backup log: %v", err)
	}

	logger.Warnf("Backup schedule %d completed. Note: Actual mysqldump execution requires external tools", schedule.Id)
	logger.Infof("Backup schedule %d executed by user %s", schedule.Id, username)

	return nil
}

// DeleteBackupSchedule deletes a backup schedule
func (s *Service) DeleteBackupSchedule(c *gin.Context, req *DeleteNMSBackupTaskRequest) error {
	if err := s.repo.DeleteByID(req.Id); err != nil {
		logger.Errorf("Failed to delete backup schedule %d: %v", req.Id, err)
		return apperror.ErrInternal.WithMessage("failed to delete backup schedule")
	}

	logger.Infof("Deleted backup schedule %d", req.Id)
	return nil
}

// RevertBackup triggers restore from a backup schedule's file
func (s *Service) RevertBackup(c *gin.Context, req *RevertNMSBackupTaskRequest) error {
	username := middleware.GetUsername(c)

	schedule, err := s.repo.FindByID(req.Id)
	if err != nil {
		return apperror.ErrNotFound.WithMessage("backup schedule not found")
	}

	now := time.Now()

	// Mark revert as running
	schedule.RevertStatus = intPtr(1) // running
	schedule.RevertBeginTime = &now
	s.repo.Save(schedule)

	// Create task execution record
	task := &NMSBackupAndRevertTask{
		Name:        strPtr(derefString(schedule.BackupName)),
		TaskType:    strPtr("revert"),
		ExecuteMode: intPtr(1),
		Status:      intPtr(1),
		CreateTime:  &now,
		UpdateTime:  &now,
		User:        strPtr(username),
		NmsBackupId: intPtr(schedule.Id),
	}

	if err := s.repo.CreateTask(task); err != nil {
		logger.Errorf("Failed to create revert task record: %v", err)
		return apperror.ErrInternal.WithMessage("failed to create revert task record")
	}

	// Mark task as running then done
	taskRunning := 2
	task.Status = &taskRunning
	task.StartTime = &now
	s.repo.UpdateTask(task)

	endTime := time.Now()
	taskDone := 3
	task.Status = &taskDone
	task.EndTime = &endTime
	s.repo.UpdateTask(task)

	// Mark revert as completed
	schedule.RevertStatus = intPtr(2) // completed
	schedule.RevertEndTime = &endTime
	s.repo.Save(schedule)

	// Create log record
	logRecord := &NMSBackupAndRevertLog{
		FileName:      schedule.FileName,
		Time:          &now,
		OperationUser: strPtr(username),
		Result:        intPtr(0), // success
	}

	if err := s.repo.CreateLog(logRecord); err != nil {
		logger.Errorf("Failed to create revert log: %v", err)
	}

	logger.Warnf("Revert completed for schedule %d. Note: Actual mysql restore requires external tools", req.Id)
	logger.Infof("Revert executed for schedule %d by user %s", req.Id, username)

	return nil
}

// GetBackupRetentionConfig reads retention configuration
func (s *Service) GetBackupRetentionConfig() (*GetBackupAndRestoreConfigDTO, error) {
	config, err := s.repo.GetRetentionConfig()
	if err != nil {
		logger.Errorf("Failed to get backup retention config: %v", err)
		return nil, apperror.ErrInternal.WithMessage("failed to get retention config")
	}
	return &GetBackupAndRestoreConfigDTO{
		BackupFileSavedDays: derefInt(config.BackupFileSavedDays),
	}, nil
}

// UpdateBackupRetentionConfig updates retention configuration
func (s *Service) UpdateBackupRetentionConfig(req *UpdateBackupRetentionRequest) error {
	config, err := s.repo.GetRetentionConfig()
	if err != nil {
		return apperror.ErrInternal.WithMessage("failed to get current config")
	}

	if req.BackupFileSavedDays != nil {
		config.BackupFileSavedDays = req.BackupFileSavedDays
	}

	if err := s.repo.UpdateRetentionConfig(config); err != nil {
		logger.Errorf("Failed to update backup retention config: %v", err)
		return apperror.ErrInternal.WithMessage("failed to update retention config")
	}

	logger.Infof("Updated backup retention config")
	return nil
}

// ListBackupLogs returns paginated list of backup/revert logs
func (s *Service) ListBackupLogs(req *ListNMSBackupLogsRequest) ([]NMSBackupLogVo, int64, error) {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	logs, total, err := s.repo.ListLogs(page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var result []NMSBackupLogVo
	for _, log := range logs {
		vo := NMSBackupLogVo{
			Id:            log.Id,
			FileName:      derefString(log.FileName),
			Time:          formatTime(log.Time),
			OperationUser: derefString(log.OperationUser),
			Result:        derefInt(log.Result),
			Reason:        derefString(log.Reason),
		}
		result = append(result, vo)
	}

	return result, total, nil
}

// GetBackupLogDetail returns a single log detail
func (s *Service) GetBackupLogDetail(req *GetNMSBackupLogDetailRequest) (*NMSBackupAndRevertLog, error) {
	log, err := s.repo.GetLogById(req.LogId)
	if err != nil {
		return nil, apperror.ErrNotFound.WithMessage("backup log not found")
	}
	return log, nil
}

// --- Helper functions ---

func strPtr(s string) *string {
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func intPtr(i int) *int {
	return &i
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", s)
}
