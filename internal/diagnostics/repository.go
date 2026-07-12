package diagnostics

import (
	"time"

	"gorm.io/gorm"
)

// Repository defines the data-access contract for diagnostics.
type Repository interface {
	FindElementRootNode(elementId int64) (string, error)
	FindElementSerialNumber(elementId int64) (string, error)
	FindParamsByElementIdAndNames(elementId int64, paramNames []string) ([]ParameterValue, error)
	FindParamsByElementIdAndNameLike(elementId int64, pattern string) ([]ParameterValue, error)
	InsertEventLog(eventType string, elementId int64, user string, status int, commandTrackData string) (int64, error)
}

// repository is the concrete GORM-backed implementation of Repository.
type repository struct {
	db *gorm.DB
}

// NewRepository creates a Repository with the given database connection.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// FindElementRootNode 查询设备的 rootNode (TR-181: "Device", TR-098: "InternetGatewayDevice")
func (r *repository) FindElementRootNode(elementId int64) (string, error) {
	var rootNode string
	err := r.db.Table("cpe_element").
		Select("root_node").
		Where("ne_neid = ? AND deleted = 0", elementId).
		Scan(&rootNode).Error
	return rootNode, err
}

// FindElementSerialNumber 查询设备的 serial_number
func (r *repository) FindElementSerialNumber(elementId int64) (string, error) {
	var sn string
	err := r.db.Table("cpe_element").
		Select("serial_number").
		Where("ne_neid = ? AND deleted = 0", elementId).
		Scan(&sn).Error
	return sn, err
}

// FindParamsByElementIdAndNames 查询指定参数名的参数值（用于获取 type 信息）
func (r *repository) FindParamsByElementIdAndNames(elementId int64, paramNames []string) ([]ParameterValue, error) {
	var params []ParameterValue
	err := r.db.Where("element_id = ? AND param_name IN ?", elementId, paramNames).Find(&params).Error
	return params, err
}

// FindParamsByElementIdAndNameLike 按参数名前缀模糊查询
func (r *repository) FindParamsByElementIdAndNameLike(elementId int64, pattern string) ([]ParameterValue, error) {
	var params []ParameterValue
	err := r.db.Where("element_id = ? AND param_name LIKE ?", elementId, pattern).Find(&params).Error
	return params, err
}

// InsertEventLog 创建 event_log 行并返回自增 ID
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
