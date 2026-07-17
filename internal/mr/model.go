package mr

import "time"

// MRData mirrors Java's station-new dao.entity.MRData (table mr_data). All 24
// MR measurements are stored as strings holding the raw encoded integers,
// exactly like Java (so downstream CSV/heatmap decoding matches).
type MRData struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementID      int64      `gorm:"column:element_id;index" json:"elementId"`
	CellID         string     `gorm:"column:cell_id" json:"cellId"`
	UeID           string     `gorm:"column:ue_id" json:"ueId"`
	AmfID          string     `gorm:"column:amf_id" json:"amfId"`
	EventTime      *time.Time `gorm:"column:event_time" json:"eventTime"`
	StartTime      *time.Time `gorm:"column:start_time;index" json:"startTime"`
	EndTime        *time.Time `gorm:"column:end_time;index" json:"endTime"`
	EventType      string     `gorm:"column:event_type" json:"eventType"`
	NRScArfcn      string     `gorm:"column:nr_sc_arfcn" json:"NRScArfcn"`
	NRScPci        string     `gorm:"column:nr_sc_pci" json:"NRScPci"`
	NRScSSRSRP     string     `gorm:"column:nr_sc_ssrsrp" json:"NRScSSRSRP"`
	NRScSSRSRQ     string     `gorm:"column:nr_sc_ssrsrq" json:"NRScSSRSRQ"`
	NRScSSSINR     string     `gorm:"column:nr_sc_sssinr" json:"NRScSSSINR"`
	NRScTadv       string     `gorm:"column:nr_sc_tadv" json:"NRScTadv"`
	NRScPHR        string     `gorm:"column:nr_sc_phr" json:"NRScPHR"`
	HAOA           string     `gorm:"column:h_aoa" json:"hAOA"`
	VAOA           string     `gorm:"column:v_aoa" json:"vAOA"`
	NRUEPlrUL      string     `gorm:"column:nrue_plr_ul" json:"NRUEPlrUL"`
	NRUEPlrDL      string     `gorm:"column:nrue_plr_dl" json:"NRUEPlrDL"`
	NRNcArfcn      string     `gorm:"column:nr_nc_arfcn" json:"NRNcArfcn"`
	NRNcPci        string     `gorm:"column:nr_nc_pci" json:"NRNcPci"`
	NRNcSSRSRP     string     `gorm:"column:nr_nc_ssrsrp" json:"NRNcSSRSRP"`
	NRNcSSRSRQ     string     `gorm:"column:nr_nc_ssrsrq" json:"NRNcSSRSRQ"`
	NRNcSSSINR     string     `gorm:"column:nr_nc_sssinr" json:"NRNcSSSINR"`
	LteNcEarfcn    string     `gorm:"column:lte_nc_earfcn" json:"LteNcEarfcn"`
	LteNcPci       string     `gorm:"column:lte_nc_pci" json:"LteNcPci"`
	LteNcRSRP      string     `gorm:"column:lte_nc_rsrp" json:"LteNcRSRP"`
	LteNcRSRQ      string     `gorm:"column:lte_nc_rsrq" json:"LteNcRSRQ"`
	PLMN           string     `gorm:"column:plmn" json:"PLMN"`
	NRScSSBIndexId string     `gorm:"column:nr_sc_ssb_index_id" json:"NRScSSBIndexId"`
	NRNcSSBIndexId string     `gorm:"column:nr_nc_ssb_index_id" json:"NRNcSSBIndexId"`
	Longitude      string     `gorm:"column:longitude" json:"Longitude"`
	Latitude       string     `gorm:"column:latitude" json:"Latitude"`
}

func (MRData) TableName() string { return "mr_data" }

// MRFileLog mirrors Java's MRFileLog (table mr_file_upload_log).
type MRFileLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ElementID  int64     `gorm:"column:element_id;index" json:"elementId"`
	FileName   string    `gorm:"column:file_name" json:"fileName"`
	UploadTime time.Time `gorm:"column:upload_time;index" json:"uploadTime"`
}

func (MRFileLog) TableName() string { return "mr_file_upload_log" }
