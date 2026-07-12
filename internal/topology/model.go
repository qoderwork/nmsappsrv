package topology

import "time"

// ---------------------------------------------------------------------------
// Topology response models (matching Java: LteTopologyVO, TopologyBBUVO, etc.)
// ---------------------------------------------------------------------------

// LteTopologyResponse is the top-level response for both LTE and NR topology queries.
// Java: LteTopologyVO
type LteTopologyResponse struct {
	BBU *TopologyBBU `json:"bbu"`
}

// TopologyBBU represents the BBU (base-band unit) root node.
// Java: TopologyBBUVO
type TopologyBBU struct {
	DeviceName   *string           `json:"deviceName"`
	SerialNumber *string           `json:"serialNumber"`
	Ports        []TopologyBBUPort `json:"ports"`
}

// TopologyBBUPort represents a device connected to a BBU port (first-level device).
// Java: TopologyBBUPortVO extends TopologyDevice
type TopologyBBUPort struct {
	PortIndex        int              `json:"portIndex"`
	Type             string           `json:"type"`
	SerialNumber     string           `json:"serialNumber"`
	IP               string           `json:"ip,omitempty"`
	SoftwareVersion  string           `json:"softwareVersion"`
	DeviceType       string           `json:"deviceType"`
	ConnectStatus    int              `json:"connectStatus"`
	Route            string           `json:"route"`
	NextLevelDevices []TopologyDevice `json:"nextLevelDevices"`
}

// TopologyDevice represents a cascaded/downstream device in the topology tree.
// Java: TopologyDevice
type TopologyDevice struct {
	PortIndex        int              `json:"portIndex"`
	Type             string           `json:"type"`
	SerialNumber     string           `json:"serialNumber"`
	IP               string           `json:"ip,omitempty"`
	SoftwareVersion  string           `json:"softwareVersion"`
	DeviceType       string           `json:"deviceType"`
	ConnectStatus    int              `json:"connectStatus"`
	Route            string           `json:"route"`
	NextLevelDevices []TopologyDevice `json:"nextLevelDevices"`
}

// ---------------------------------------------------------------------------
// Batch upgrade models (matching Java: BatchUpgradeEUAndRUDTO, etc.)
// ---------------------------------------------------------------------------

// BatchUpgradeRequest is the request body for batch EU/RU upgrade.
// Java: BatchUpgradeEUAndRUDTO
type BatchUpgradeRequest struct {
	ElementId int64                  `json:"elementId" binding:"required"`
	Devices   []DeviceAndUpgradeFile `json:"devices" binding:"required"`
}

// DeviceAndUpgradeFile maps a device route to an upgrade file.
// Java: DeviceAndUpgradeFileDTO
type DeviceAndUpgradeFile struct {
	Route         string `json:"route" binding:"required"`
	UpgradeFileId int    `json:"upgradeFileId" binding:"required"`
}

// ---------------------------------------------------------------------------
// Batch upgrade log query / response
// ---------------------------------------------------------------------------

// ListBatchUpgradeLogQuery is the query filter for listing batch upgrade logs.
// Java: ListBatchUpgradeLogQuery
type ListBatchUpgradeLogQuery struct {
	ElementId     int64      `json:"elementId" binding:"required"`
	OperationUser string     `json:"operationUser"`
	StartTime     *time.Time `json:"startTime"`
	EndTime       *time.Time `json:"endTime"`
	Page          int        `json:"page"`
	PageSize      int        `json:"pageSize"`
}

// ListBatchUpgradeLogVO is one row in the batch upgrade log listing.
// Java: ListBatchUpgradeLogVO
type ListBatchUpgradeLogVO struct {
	OperationTime  *time.Time      `json:"operationTime"`
	DownloadedTime *time.Time      `json:"downloadedTime"`
	UpgradedTime   *time.Time      `json:"upgradedTime"`
	Result         *int            `json:"result"`
	OperationUser  string          `json:"operationUser"`
	FaultInfo      string          `json:"faultInfo"`
	VersionInfo    []VersionChange `json:"versionInfo"`
}

// VersionChange describes a single device's version transition in a batch upgrade.
// Java: VersionChangeVO
type VersionChange struct {
	SerialNumber    string `json:"serialNumber"`
	OriginalVersion string `json:"originalVersion"`
	UpgradedVersion string `json:"upgradedVersion"`
	UpgradeFileName string `json:"upgradeFileName"`
	FaultInfo       string `json:"faultInfo"`
}

// EUAndRUUpgradeDTO stores per-device upgrade info serialized as JSON in the log.
// Java: EUAndRUUpgradeDTO
type EUAndRUUpgradeDTO struct {
	SerialNumber  string `json:"serialNumber"`
	Route         string `json:"route"`
	Version       string `json:"version"`
	UpgradeFileId int    `json:"upgradeFileId"`
	FaultInfo     string `json:"faultInfo,omitempty"`
}

// LongIdRequest is a generic request carrying a single element/device ID.
// Java: LongIdDTO
type LongIdRequest struct {
	Id int64 `json:"id" binding:"required"`
}
