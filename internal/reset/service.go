package reset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/internal/mq"
	"nmsappsrv/internal/opmsg"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// Repository defines the data-access contract for reset management.
type Repository interface {
	CreateTask(t *ResetTask) error
	FindTaskByID(id int) (*ResetTask, error)
	DeleteTask(id int) error
	UpdateTask(t *ResetTask) error
	TaskNameExists(tenancyId int, name string) bool
	FindElementIdsByGroup(groupIds []string) ([]int64, error)
	FindDueTimedTasks(before time.Time) ([]ResetTask, error)
	FindElementInfo(elementId int64) (sn string, deviceType string, err error)
	InsertEventLog(eventType string, elementId int64, user string, status int, faultInfo string) (int64, error)
	CreateTaskToEventLog(taskId int, eventLogId int64, taskType string) error
	IsDeviceInUpgrade(elementId int64) bool
	ListTasks(tenancyId int, query ListResetTaskQuery) ([]ResetTaskVO, int64, error)
	ListTaskResults(query ListResetTaskResultQuery) ([]ResetTaskResultVO, int64, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// Service defines the business-logic contract for reset management.
type Service interface {
	AddResetTask(req *AddResetTaskRequest, tenancyId int, username string) (int, error)
	DeleteResetTask(id int) error
	StartResetTask(id int, username string) error
	CancelResetTask(id int) error
	TriggerDueTimedTasks(ctx context.Context) (int, error)
	ListTasks(tenancyId int, query ListResetTaskQuery) ([]ResetTaskVO, int64, error)
	ListTaskResults(query ListResetTaskResultQuery) ([]ResetTaskResultVO, int64, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a new reset service.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// validDeviceTypes mirrors Java's reboot/reset deviceType whitelist
// (Java rejects anything outside {CPE, BaseStation} with error 10031).
var validDeviceTypes = map[string]bool{
	"CPE":         true,
	"BaseStation": true,
}

func isValidDeviceType(dt string) bool {
	return validDeviceTypes[dt]
}

// ---------- Repository methods ----------

func (r *repository) CreateTask(t *ResetTask) error {
	return r.db.Create(t).Error
}

func (r *repository) FindTaskByID(id int) (*ResetTask, error) {
	var t ResetTask
	err := r.db.First(&t, id).Error
	return &t, err
}

func (r *repository) DeleteTask(id int) error {
	return r.db.Delete(&ResetTask{}, id).Error
}

func (r *repository) UpdateTask(t *ResetTask) error {
	return r.db.Save(t).Error
}

func (r *repository) TaskNameExists(tenancyId int, name string) bool {
	var count int64
	r.db.Model(&ResetTask{}).Where("tenancy_id = ? AND name = ?", tenancyId, name).Count(&count)
	return count > 0
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

// FindDueTimedTasks returns scheduled (execute_mode=3) Waiting tasks whose
// trigger time has already passed.
func (r *repository) FindDueTimedTasks(before time.Time) ([]ResetTask, error) {
	var tasks []ResetTask
	err := r.db.Where("execute_mode = ? AND status = ? AND trigger_time IS NOT NULL AND trigger_time <= ?", 3, 1, before).
		Find(&tasks).Error
	return tasks, err
}

func (r *repository) FindElementInfo(elementId int64) (sn string, deviceType string, err error) {
	var row struct {
		SN         string `gorm:"column:serial_number"`
		DeviceType string `gorm:"column:device_type"`
	}
	err = r.db.Table("cpe_element").
		Select("serial_number, device_type").
		Where("ne_neid = ? AND deleted = 0", elementId).
		Scan(&row).Error
	return row.SN, row.DeviceType, err
}

func (r *repository) InsertEventLog(eventType string, elementId int64, user string, status int, faultInfo string) (int64, error) {
	row := struct {
		Id               int64     `gorm:"primaryKey;autoIncrement"`
		EventType        string    `gorm:"column:event_type;type:varchar(255)"`
		OperationTime    time.Time `gorm:"column:operation_time"`
		User             string    `gorm:"column:user;type:varchar(255)"`
		ElementId        int64     `gorm:"column:element_id"`
		Status           int       `gorm:"column:status"`
		FaultInfo        string    `gorm:"column:fault_info;type:varchar(1024)"`
		CommandTrackData string    `gorm:"column:command_track_data;type:longtext"`
	}{
		EventType:     eventType,
		OperationTime: time.Now(),
		User:          user,
		ElementId:     elementId,
		Status:        status,
		FaultInfo:     faultInfo,
	}
	if err := r.db.Table("event_log").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

func (r *repository) CreateTaskToEventLog(taskId int, eventLogId int64, taskType string) error {
	rel := TaskToEventLog{TaskId: taskId, EventLogId: eventLogId, TaskType: taskType}
	return r.db.Create(&rel).Error
}

func (r *repository) IsDeviceInUpgrade(elementId int64) bool {
	var count int64
	r.db.Table("upgrade_log").Where("element_id = ? AND status IN (1,2)", elementId).Count(&count)
	if count > 0 {
		return true
	}
	r.db.Table("manual_upgrade_log").Where("element_id = ? AND status IN (1,2)", elementId).Count(&count)
	return count > 0
}

func (r *repository) ListTasks(tenancyId int, query ListResetTaskQuery) ([]ResetTaskVO, int64, error) {
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	baseSQL := `
		SELECT rt.id, rt.name, rt.user, rt.operation_time, rt.status,
		       rt.start_time, rt.end_time, rt.tenancy_id,
		       COUNT(tel.id) AS total_count,
		       SUM(CASE WHEN el.status = 3 THEN 1 ELSE 0 END) AS success_count,
		       MAX(el.command_response_time) AS last_response_time
		FROM reset_task rt
		LEFT JOIN task_to_event_log tel ON tel.task_id = rt.id AND tel.task_type = 'reset'
		LEFT JOIN event_log el ON el.id = tel.event_log_id
		WHERE rt.tenancy_id = ?`
	args := []interface{}{tenancyId}

	if query.TaskName != "" {
		baseSQL += " AND rt.name LIKE ?"
		args = append(args, "%"+query.TaskName+"%")
	}
	if query.DeviceType != "" {
		baseSQL += " AND rt.device_type = ?"
		args = append(args, query.DeviceType)
	}
	if query.StartTime != nil {
		if t, err := time.Parse(time.RFC3339, *query.StartTime); err == nil {
			baseSQL += " AND rt.operation_time >= ?"
			args = append(args, t)
		}
	}
	if query.EndTime != nil {
		if t, err := time.Parse(time.RFC3339, *query.EndTime); err == nil {
			baseSQL += " AND rt.operation_time <= ?"
			args = append(args, t)
		}
	}

	countSQL := "SELECT COUNT(*) FROM reset_task rt WHERE rt.tenancy_id = ?"
	countArgs := []interface{}{tenancyId}
	if query.TaskName != "" {
		countSQL += " AND rt.name LIKE ?"
		countArgs = append(countArgs, "%"+query.TaskName+"%")
	}
	if query.DeviceType != "" {
		countSQL += " AND rt.device_type = ?"
		countArgs = append(countArgs, query.DeviceType)
	}
	var total int64
	r.db.Raw(countSQL, countArgs...).Scan(&total)

	dataSQL := baseSQL + " GROUP BY rt.id ORDER BY rt.operation_time DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	type taskRow struct {
		Id               int        `gorm:"column:id"`
		Name             string     `gorm:"column:name"`
		User             string     `gorm:"column:user"`
		OperationTime    time.Time  `gorm:"column:operation_time"`
		Status           int        `gorm:"column:status"`
		StartTime        *time.Time `gorm:"column:start_time"`
		EndTime          *time.Time `gorm:"column:end_time"`
		TenancyId        int        `gorm:"column:tenancy_id"`
		TotalCount       int64      `gorm:"column:total_count"`
		SuccessCount     int64      `gorm:"column:success_count"`
		LastResponseTime *time.Time `gorm:"column:last_response_time"`
	}
	var rows []taskRow
	if err := r.db.Raw(dataSQL, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	taskIds := make([]int, len(rows))
	for i, row := range rows {
		taskIds[i] = row.Id
	}
	failMap := make(map[int]bool)
	pendingMap := make(map[int]bool)
	if len(taskIds) > 0 {
		type idRow struct {
			TaskId int `gorm:"column:task_id"`
		}
		var failRows []idRow
		r.db.Raw(`
			SELECT DISTINCT tel.task_id FROM task_to_event_log tel
			JOIN event_log el ON el.id = tel.event_log_id
			WHERE tel.task_id IN ? AND tel.task_type = 'reset' AND el.status IN (4,5,6)
		`, taskIds).Scan(&failRows)
		for _, fr := range failRows {
			failMap[fr.TaskId] = true
		}
		var pendRows []idRow
		r.db.Raw(`
			SELECT DISTINCT tel.task_id FROM task_to_event_log tel
			JOIN event_log el ON el.id = tel.event_log_id
			WHERE tel.task_id IN ? AND tel.task_type = 'reset' AND el.status IN (1,2)
		`, taskIds).Scan(&pendRows)
		for _, pr := range pendRows {
			pendingMap[pr.TaskId] = true
		}
	}

	tenancyNames := r.getTenancyNames()
	vos := make([]ResetTaskVO, len(rows))
	for i, row := range rows {
		vo := ResetTaskVO{
			Id:            row.Id,
			Name:          row.Name,
			User:          row.User,
			OperationTime: row.OperationTime,
			Status:        row.Status,
			StartTime:     row.StartTime,
			EndTime:       row.LastResponseTime,
			TenancyName:   tenancyNames[row.TenancyId],
		}
		if row.TotalCount > 0 {
			vo.Progress = fmt.Sprintf("%d/%d", row.SuccessCount, row.TotalCount)
		}
		if row.Status == 1 || row.Status == 4 {
			vo.Results = nil
		} else if failMap[row.Id] {
			res := 2
			vo.Results = &res
		} else if !pendingMap[row.Id] && row.TotalCount > 0 {
			res := 1
			vo.Results = &res
		}
		vos[i] = vo
	}
	return vos, total, nil
}

func (r *repository) ListTaskResults(query ListResetTaskResultQuery) ([]ResetTaskResultVO, int64, error) {
	page, pageSize := query.Page, query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	baseSQL := `
		SELECT ce.serial_number, ce.device_name, el.status, el.fault_info,
		       el.command_response_time, ce.ne_neid
		FROM task_to_event_log tel
		JOIN event_log el ON el.id = tel.event_log_id
		JOIN cpe_element ce ON ce.ne_neid = el.element_id
		WHERE tel.task_id = ? AND tel.task_type = 'reset'`
	args := []interface{}{query.TaskId}
	if query.SerialNumber != "" {
		baseSQL += " AND ce.serial_number LIKE ?"
		args = append(args, "%"+query.SerialNumber+"%")
	}

	var total int64
	countSQL := `SELECT COUNT(*) FROM task_to_event_log tel
		JOIN event_log el ON el.id = tel.event_log_id
		JOIN cpe_element ce ON ce.ne_neid = el.element_id
		WHERE tel.task_id = ? AND tel.task_type = 'reset'`
	countArgs := []interface{}{query.TaskId}
	if query.SerialNumber != "" {
		countSQL += " AND ce.serial_number LIKE ?"
		countArgs = append(countArgs, "%"+query.SerialNumber+"%")
	}
	r.db.Raw(countSQL, countArgs...).Scan(&total)

	baseSQL += " ORDER BY el.operation_time DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	type resultRow struct {
		SerialNumber        string     `gorm:"column:serial_number"`
		DeviceName          string     `gorm:"column:device_name"`
		Status              int        `gorm:"column:status"`
		FaultInfo           string     `gorm:"column:fault_info"`
		CommandResponseTime *time.Time `gorm:"column:command_response_time"`
		NeNeid              int64      `gorm:"column:ne_neid"`
	}
	var rows []resultRow
	if err := r.db.Raw(baseSQL, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	vos := make([]ResetTaskResultVO, len(rows))
	for i, row := range rows {
		vo := ResetTaskResultVO{
			SerialNumber:  row.SerialNumber,
			DeviceName:    row.DeviceName,
			Status:        row.Status,
			FailureReason: row.FaultInfo,
			Time:          row.CommandResponseTime,
			ElementId:     row.NeNeid,
		}
		if row.Status == 3 {
			res := 1
			vo.Results = &res
		} else if row.Status == 4 || row.Status == 5 || row.Status == 6 {
			res := 2
			vo.Results = &res
		}
		vos[i] = vo
	}
	return vos, total, nil
}

func (r *repository) getTenancyNames() map[int]string {
	m := make(map[int]string)
	type row struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	var rows []row
	r.db.Table("tenancy").Select("id, name").Scan(&rows)
	for _, row := range rows {
		m[row.Id] = row.Name
	}
	return m
}

// ---------- Service methods ----------

// AddResetTask creates a new reset task and dispatches commands if immediate.
func (s *service) AddResetTask(req *AddResetTaskRequest, tenancyId int, username string) (int, error) {
	if s.repo.TaskNameExists(tenancyId, req.Name) {
		return 0, fmt.Errorf("task name already exists")
	}
	if !isValidDeviceType(req.DeviceType) {
		return 0, fmt.Errorf("invalid deviceType %q: must be CPE or BaseStation", req.DeviceType)
	}

	elementIds := req.ElementIds
	if req.Scope == "deviceGroup" && len(req.DeviceGroupIds) > 0 {
		groupIds, err := s.repo.FindElementIdsByGroup(req.DeviceGroupIds)
		if err != nil {
			return 0, fmt.Errorf("resolve device groups: %w", err)
		}
		elementIds = append(elementIds, groupIds...)
	}
	if len(elementIds) == 0 {
		return 0, fmt.Errorf("no devices selected")
	}

	now := time.Now()
	task := &ResetTask{
		Name:           req.Name,
		User:           username,
		OperationTime:  now,
		ExecuteMode:    req.ExecuteMode,
		TenancyId:      tenancyId,
		ElementIds:     marshalElementIds(elementIds),
		DeviceType:     req.DeviceType,
		Scope:          req.Scope,
		DeviceGroupIds: marshalGroupIds(req.DeviceGroupIds),
	}

	if req.ExecuteMode == 1 {
		task.Status = 2
		task.StartTime = &now
	} else {
		task.Status = 1
	}
	if req.ExecuteMode == 3 && req.TriggerTime != nil {
		if t, err := time.Parse(time.RFC3339, *req.TriggerTime); err == nil {
			task.TriggerTime = &t
		}
	}

	if err := s.repo.CreateTask(task); err != nil {
		return 0, err
	}
	if req.ExecuteMode == 1 {
		s.dispatchReset(task, elementIds, username)
	}
	return task.Id, nil
}

func (s *service) DeleteResetTask(id int) error {
	return s.repo.DeleteTask(id)
}

func (s *service) StartResetTask(id int, username string) error {
	task, err := s.repo.FindTaskByID(id)
	if err != nil {
		return err
	}
	if task.Status != 1 {
		return fmt.Errorf("task already started or completed")
	}

	ctx := context.Background()
	lockKey := fmt.Sprintf("reset_task_start_%d", id)
	if !redis.Lock(ctx, lockKey, 60*time.Second) {
		return fmt.Errorf("task is being started by another request")
	}
	defer redis.Unlock(ctx, lockKey)

	now := time.Now()
	task.Status = 2
	task.StartTime = &now
	s.repo.UpdateTask(task)

	elementIds := parseElementIds(task.ElementIds)
	s.dispatchReset(task, elementIds, username)
	return nil
}

func (s *service) CancelResetTask(id int) error {
	task, err := s.repo.FindTaskByID(id)
	if err != nil {
		return err
	}
	task.Status = 4
	return s.repo.UpdateTask(task)
}

func (s *service) ListTasks(tenancyId int, query ListResetTaskQuery) ([]ResetTaskVO, int64, error) {
	return s.repo.ListTasks(tenancyId, query)
}

func (s *service) ListTaskResults(query ListResetTaskResultQuery) ([]ResetTaskResultVO, int64, error) {
	return s.repo.ListTaskResults(query)
}

// ---------- dispatch ----------

func (s *service) dispatchReset(task *ResetTask, elementIds []int64, username string) {
	ctx := context.Background()

	for _, eid := range elementIds {
		sn, _, err := s.repo.FindElementInfo(eid)
		if err != nil {
			logger.Errorf("reset: find element %d: %v", eid, err)
			continue
		}

		// Blacklist check
		blKey := fmt.Sprintf("black_list_%s%s", task.DeviceType, sn)
		blVal, _ := redis.Get(ctx, blKey)
		if blVal == "y" {
			logger.Infof("reset: device %s is blacklisted, skipping", sn)
			continue
		}

		// Upgrade conflict
		if s.repo.IsDeviceInUpgrade(eid) {
			elId, err := s.repo.InsertEventLog("FactoryReset", eid, username, 5, "Device is in upgrade")
			if err == nil {
				s.repo.CreateTaskToEventLog(task.Id, elId, "reset")
			}
			continue
		}

		// Create event_log (pending)
		elId, err := s.repo.InsertEventLog("FactoryReset", eid, username, 1, "")
		if err != nil {
			logger.Errorf("reset: create event_log for %d: %v", eid, err)
			continue
		}
		s.repo.CreateTaskToEventLog(task.Id, elId, "reset")

		// Push to operation_queue
		now := time.Now()
		msg := opmsg.Message{
			EventType:      "FactoryReset", // Java EventType.FACTORY_RESET
			NeNeid:         eid,
			Operation:      "FactoryReset",
			OperationUser:  username,
			CommandTrackId: elId,
			ExpiredAt:      now.Add(5 * time.Minute).UnixMilli(),
		}
		msgJSON, _ := msg.Marshal()
		if err := redis.LPush(ctx, mq.OperationQueue, string(msgJSON)); err != nil {
			logger.Errorf("reset: push to queue for %d: %v", eid, err)
		}
	}
}

// TriggerDueTimedTasks fires any scheduled (ExecuteMode==3) reset tasks whose
// trigger time has passed and that are still Waiting. Mirrors Java's Quartz
// ResetTaskJob. Returns the number of tasks dispatched.
func (s *service) TriggerDueTimedTasks(ctx context.Context) (int, error) {
	tasks, err := s.repo.FindDueTimedTasks(time.Now())
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range tasks {
		task := &tasks[i]
		lockKey := fmt.Sprintf("reset_timed_%d", task.Id)
		if !redis.Lock(ctx, lockKey, 60*time.Second) {
			continue
		}
		// Re-check status under lock to avoid double-dispatch across ticks.
		fresh, ferr := s.repo.FindTaskByID(task.Id)
		if ferr != nil || fresh.Status != 1 {
			redis.Unlock(ctx, lockKey)
			continue
		}
		now := time.Now()
		fresh.Status = 2
		fresh.StartTime = &now
		if err := s.repo.UpdateTask(fresh); err != nil {
			logger.Errorf("reset: mark scheduled task %d executing: %v", task.Id, err)
			redis.Unlock(ctx, lockKey)
			continue
		}
		redis.Unlock(ctx, lockKey)
		s.dispatchReset(fresh, parseElementIds(fresh.ElementIds), fresh.User)
		n++
	}
	return n, nil
}

// ---------- helpers ----------

func marshalElementIds(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

func marshalGroupIds(ids []string) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

func parseElementIds(jsonStr string) []int64 {
	var ids []int64
	if jsonStr != "" {
		json.Unmarshal([]byte(jsonStr), &ids)
	}
	return ids
}

// newService creates a Service backed by the given mock Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
