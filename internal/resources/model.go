package resources

// systemConfigModel maps to the system_config table (shared table, local definition)
type systemConfigModel struct {
	Id     string  `gorm:"primaryKey;column:id;type:varchar(255)"`
	Config *string `gorm:"column:config;type:longtext"`
}

func (systemConfigModel) TableName() string { return "system_config" }

// ResourcesVO represents CPU and memory usage
type ResourcesVO struct {
	CPU       float64 `json:"cpu"`
	Mem       float64 `json:"mem"`
	Timestamp string  `json:"timestamp"`
}

// TableStatusVO represents MySQL table status
type TableStatusVO struct {
	TableName string  `json:"tableName"`
	TableRows int64   `json:"tableRows"`
	SizeGB    float64 `json:"sizeGB"`
}

// DiskUsageVO represents disk partition usage
type DiskUsageVO struct {
	Filesystem string `json:"filesystem"`
	Size       string `json:"size"`
	Used       string `json:"used"`
	Avail      string `json:"avail"`
	UsePercent string `json:"usePercent"`
	MountPoint string `json:"mountPoint"`
}

// ThresholdConfig represents alarm threshold configuration
type ThresholdConfig struct {
	CPU       float64 `json:"cpu"`
	Mem       float64 `json:"mem"`
	Disk      float64 `json:"disk"`
	DiskClear float64 `json:"diskClear"`
	Table     float64 `json:"table"`
}
