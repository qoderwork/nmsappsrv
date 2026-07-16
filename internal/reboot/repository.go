package reboot

import (
	"encoding/json"
	"fmt"
	"time"

	"nmsappsrv/pkg/baserepo"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for reboot management.
type Repository interface {
	// Generic BaseRepository[RebootTask, int] methods.
	Create(entity *RebootTask) error
	Save(entity *RebootTask) error
	FindByID(id int) (*RebootTask, error)
	DeleteByID(id int) error
	DeleteByIDs(ids []int) error
	SoftDelete(id int) error
	UpdateFields(id int, fields map[string]interface{}) error
	FindAll(query *gorm.DB) ([]RebootTask, error)
	Count(query *gorm.DB) (int64, error)
	FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*baserepo.PageResult[RebootTask], error)

	// Module-specific methods.
	TaskNameExists(tenancyId int, name string) bool
	FindElementIdsByGroup(groupIds []string) ([]int64, error)
	FindDueTimedTasks(before time.Time) ([]RebootTask, error)
	FindElementInfo(elementId int64) (sn string, deviceType string, err error)
	InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error)
	CreateTaskToEventLog(taskId int, eventLogId int64, taskType string) error
	IsDeviceInUpgrade(elementId int64) bool
	ListTasks(tenancyId int, query ListRebootTaskQuery) ([]RebootTaskVO, int64, error)
	ListTaskResults(query ListRebootTaskResultQuery) ([]RebootTaskResultVO, int64, error)
}

// repository is the concrete GORM-backed implementation of Repository.
// It embeds BaseRepository[RebootTask, int] for standard CRUD.
type repository struct {
	*baserepo.BaseRepository[RebootTask, int]
	db *gorm.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		BaseRepository: baserepo.New[RebootTask, int](db, "id"),
		db:             db,
	}
}

// ---------------------------------------------------------------------------
// RebootTask – module-specific queries (base provides Create/Save/FindByID/DeleteByID)
// ---------------------------------------------------------------------------

// TaskNameExists checks if a task name already exists for the given tenancy.
func (r *repository) TaskNameExists(tenancyId int, name string) bool {
	var count int64
	r.db.Model(&RebootTask{}).Where("tenancy_id = ? AND name = ?", tenancyId, name).Count(&count)
	return count > 0
}

// FindElementIdsByGroup returns ne_neid list for the given device group IDs.
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
func (r *repository) FindDueTimedTasks(before time.Time) ([]RebootTask, error) {
	var tasks []RebootTask
	err := r.db.Where("execute_mode = ? AND status = ? AND trigger_time IS NOT NULL AND trigger_time <= ?", 3, 1, before).
		Find(&tasks).Error
	return tasks, err
}

// FindElementInfo returns serial_number and device_type for a given element.
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

// InsertEventLog creates an event_log row and returns its auto-increment ID.
func (r *repository) InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error) {
	row := struct {
		Id               int64     `gorm:"primaryKey;autoIncrement"`
		EventType        string    `gorm:"column:event_type;type:varchar(255)"`
		OperationTime    time.Time `gorm:"column:operation_time"`
		User             string    `gorm:"column:user;type:varchar(255)"`
		ElementId        int64     `gorm:"column:element_id"`
		Status           int       `gorm:"column:status"`
		CommandTrackData string    `gorm:"column:command_track_data;type:longtext"`
	}{
		EventType:        eventType,
		OperationTime:    time.Now(),
		User:             user,
		ElementId:        elementId,
		Status:           status,
		CommandTrackData: commandTrackData,
	}
	if err := r.db.Table("event_log").Create(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

// CreateTaskToEventLog links a task to an event_log entry.
func (r *repository) CreateTaskToEventLog(taskId int, eventLogId int64, taskType string) error {
	rel := TaskToEventLog{
		TaskId:     taskId,
		EventLogId: eventLogId,
		TaskType:   taskType,
	}
	return r.db.Create(&rel).Error
}

// IsDeviceInUpgrade checks if a device currently has an active upgrade.
func (r *repository) IsDeviceInUpgrade(elementId int64) bool {
	var count int64
	r.db.Table("upgrade_log").
		Where("element_id = ? AND status IN (1,2)", elementId).
		Count(&count)
	if count > 0 {
		return true
	}
	r.db.Table("manual_upgrade_log").
		Where("element_id = ? AND status IN (1,2)", elementId).
		Count(&count)
	return count > 0
}

// ListTasks returns paginated reboot tasks with computed progress.
func (r *repository) ListTasks(tenancyId int, query ListRebootTaskQuery) ([]RebootTaskVO, int64, error) {
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
		FROM reboot_task rt
		LEFT JOIN task_to_event_log tel ON tel.task_id = rt.id AND tel.task_type IN ('reboot','softReboot')
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

	// Count total
	countSQL := "SELECT COUNT(*) FROM reboot_task rt WHERE rt.tenancy_id = ?"
	countArgs := []interface{}{tenancyId}
	if query.TaskName != "" {
		countSQL += " AND rt.name LIKE ?"
		countArgs = append(countArgs, "%"+query.TaskName+"%")
	}
	if query.DeviceType != "" {
		countSQL += " AND rt.device_type = ?"
		countArgs = append(countArgs, query.DeviceType)
	}
	if query.StartTime != nil {
		if t, err := time.Parse(time.RFC3339, *query.StartTime); err == nil {
			countSQL += " AND rt.operation_time >= ?"
			countArgs = append(countArgs, t)
		}
	}
	if query.EndTime != nil {
		if t, err := time.Parse(time.RFC3339, *query.EndTime); err == nil {
			countSQL += " AND rt.operation_time <= ?"
			countArgs = append(countArgs, t)
		}
	}

	var total int64
	r.db.Raw(countSQL, countArgs...).Scan(&total)

	// Data query
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

	// Get failure and pending counts per task
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
			SELECT DISTINCT tel.task_id
			FROM task_to_event_log tel
			JOIN event_log el ON el.id = tel.event_log_id
			WHERE tel.task_id IN ? AND tel.task_type IN ('reboot','softReboot')
			  AND el.status IN (4,5,6)
		`, taskIds).Scan(&failRows)
		for _, fr := range failRows {
			failMap[fr.TaskId] = true
		}

		var pendRows []idRow
		r.db.Raw(`
			SELECT DISTINCT tel.task_id
			FROM task_to_event_log tel
			JOIN event_log el ON el.id = tel.event_log_id
			WHERE tel.task_id IN ? AND tel.task_type IN ('reboot','softReboot')
			  AND el.status IN (1,2)
		`, taskIds).Scan(&pendRows)
		for _, pr := range pendRows {
			pendingMap[pr.TaskId] = true
		}
	}

	tenancyNames := r.getTenancyNames()

	vos := make([]RebootTaskVO, len(rows))
	for i, row := range rows {
		vo := RebootTaskVO{
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

// ListTaskResults returns per-device results for a reboot task.
func (r *repository) ListTaskResults(query ListRebootTaskResultQuery) ([]RebootTaskResultVO, int64, error) {
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
		WHERE tel.task_id = ? AND tel.task_type IN ('reboot','softReboot')`
	args := []interface{}{query.TaskId}

	if query.SerialNumber != "" {
		baseSQL += " AND ce.serial_number LIKE ?"
		args = append(args, "%"+query.SerialNumber+"%")
	}

	// Count
	var total int64
	countSQL := `SELECT COUNT(*) FROM task_to_event_log tel
		JOIN event_log el ON el.id = tel.event_log_id
		JOIN cpe_element ce ON ce.ne_neid = el.element_id
		WHERE tel.task_id = ? AND tel.task_type IN ('reboot','softReboot')`
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

	vos := make([]RebootTaskResultVO, len(rows))
	for i, row := range rows {
		vo := RebootTaskResultVO{
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

// ---------- helpers ----------

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

// ParseElementIds deserializes the JSON element_ids column.
func ParseElementIds(jsonStr string) []int64 {
	var ids []int64
	if jsonStr != "" {
		json.Unmarshal([]byte(jsonStr), &ids)
	}
	return ids
}

// MarshalElementIds serializes element IDs to JSON.
func MarshalElementIds(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

// MarshalGroupIds serializes group IDs to JSON.
func MarshalGroupIds(ids []string) string {
	b, _ := json.Marshal(ids)
	return string(b)
}
