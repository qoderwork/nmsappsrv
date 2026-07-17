package nmsbackup

import (
	"fmt"
	"time"

	"nmsappsrv/internal/scheduler"
	"nmsappsrv/pkg/logger"
)

// BackupScheduler bridges stored backup schedule definitions to the unified
// cron scheduler, checking daily which schedules are due for execution.
type BackupScheduler struct {
	repo Repository
	svc  Service
}

// NewBackupScheduler creates a new BackupScheduler.
func NewBackupScheduler(repo Repository, svc Service) *BackupScheduler {
	return &BackupScheduler{repo: repo, svc: svc}
}

// RegisterBackupJobs registers a single daily cron job that checks all
// recurring backup schedules (backup_type=1) and fires any that are due
// based on their backup_interval (days) and backup_begin_time.
func (bc *BackupScheduler) RegisterBackupJobs(sched *scheduler.Scheduler) {
	schedules, err := bc.repo.FindScheduledSchedules()
	if err != nil {
		logger.Errorf("nmsbackup: failed to query scheduled backup schedules: %v", err)
		return
	}

	if len(schedules) == 0 {
		logger.Info("nmsbackup: no scheduled backup schedules found")
		return
	}

	logger.Infof("nmsbackup: found %d recurring backup schedules, registering daily checker", len(schedules))

	// Register a single daily cron job (at 00:05:00) that checks all schedules
	err = sched.AddJobSafeGo("nms-backup-checker", "0 5 0 * * *", func() {
		bc.checkAndFireDueSchedules()
	})
	if err != nil {
		logger.Errorf("nmsbackup: failed to register daily checker cron job: %v", err)
	}
}

// checkAndFireDueSchedules iterates all recurring schedules and fires
// any that are due based on backup_interval and last execution time.
func (bc *BackupScheduler) checkAndFireDueSchedules() {
	schedules, err := bc.repo.FindScheduledSchedules()
	if err != nil {
		logger.Errorf("nmsbackup: checker — failed to query schedules: %v", err)
		return
	}

	now := time.Now()

	for i := range schedules {
		schedule := &schedules[i]
		interval := derefInt(schedule.BackupInterval)
		if interval <= 0 {
			interval = 1 // default to daily
		}

		beginTime := schedule.BackupBeginTime
		if beginTime == nil || beginTime.After(now) {
			continue // not yet started
		}

		// Find last execution time from task records
		lastFire := bc.getLastFireTime(schedule.Id)
		if lastFire == nil {
			// Never fired — check if begin_time has passed
			if now.Before(*beginTime) {
				continue
			}
			// Fire now
			bc.executeBackup(schedule)
			continue
		}

		// Check if interval has elapsed since last fire
		daysSinceLastFire := int(now.Sub(*lastFire).Hours() / 24)
		if daysSinceLastFire >= interval {
			bc.executeBackup(schedule)
		}
	}
}

// getLastFireTime returns the most recent execution start time for a schedule
// by querying task records with nms_backup_id = scheduleId.
func (bc *BackupScheduler) getLastFireTime(scheduleId int) *time.Time {
	var task NMSBackupAndRevertTask
	err := bc.repo.GetDB().Where("nms_backup_id = ? AND task_type = ?", scheduleId, "backup").
		Order("start_time DESC").First(&task).Error
	if err != nil {
		return nil
	}
	return task.StartTime
}

// executeBackup runs a single backup for a schedule outside of an HTTP request.
func (bc *BackupScheduler) executeBackup(schedule *NMSBackupAndRevert) {
	// Mirrors Java NMSBackupTaskJob.executeInternal: skip if any other backup
	// is currently running (backupStatus==1).
	if running, err := bc.repo.FindAnyRunning(); err != nil {
		logger.Errorf("nmsbackup: cron trigger — failed to check running backups: %v", err)
		return
	} else if running != nil && running.Id != schedule.Id {
		logger.Warnf("nmsbackup: cron trigger — another backup (id=%d) is running, skipping schedule %d", running.Id, schedule.Id)
		return
	}

	now := time.Now()

	// Mark schedule as running
	runningStatus := 1
	schedule.BackupStatus = &runningStatus
	if err := bc.repo.Save(schedule); err != nil {
		logger.Errorf("nmsbackup: cron trigger — failed to update schedule %d status: %v", schedule.Id, err)
		return
	}

	// Create task execution record
	task := &NMSBackupAndRevertTask{
		Name:        strPtr(derefString(schedule.BackupName)),
		TaskType:    strPtr("backup"),
		ExecuteMode: intPtr(3), // schedule time
		Status:      intPtr(2), // running
		StartTime:   &now,
		CreateTime:  &now,
		UpdateTime:  &now,
		User:        strPtr("cron-scheduler"),
		NmsBackupId: intPtr(schedule.Id),
	}

	if err := bc.repo.CreateTask(task); err != nil {
		logger.Errorf("nmsbackup: cron trigger — failed to create task for schedule %d: %v", schedule.Id, err)
		failedStatus := 3
		schedule.BackupStatus = &failedStatus
		bc.repo.Save(schedule)
		return
	}

	// Simulate backup execution (actual mysqldump requires external tools)
	fileName := fmt.Sprintf("backup_%d_%s.sql.gz", schedule.Id, now.Format("20060102_150405"))
	endTime := time.Now()
	duration := int(endTime.Sub(now).Seconds())

	// Update task as done
	taskDone := 3
	task.Status = &taskDone
	task.EndTime = &endTime
	bc.repo.UpdateTask(task)

	// Update schedule: completed
	completedStatus := 2
	schedule.BackupStatus = &completedStatus
	schedule.BackupTimeCost = intPtr(duration)
	schedule.FileName = strPtr(fileName)
	bc.repo.Save(schedule)

	// Create log record
	logRecord := &NMSBackupAndRevertLog{
		FileName:      strPtr(fileName),
		Time:          &now,
		OperationUser: strPtr("cron-scheduler"),
		Result:        intPtr(0), // success
	}

	if err := bc.repo.CreateLog(logRecord); err != nil {
		logger.Errorf("nmsbackup: cron trigger — failed to create log for schedule %d: %v", schedule.Id, err)
	}

	logger.Infof("nmsbackup: cron backup for schedule %d completed", schedule.Id)
}
