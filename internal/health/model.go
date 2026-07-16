package health

import "time"

// HAComponentStatus represents HA component status report
type HAComponentStatus struct {
	Hostname      string `json:"hostname"`
	ComponentName string `json:"componentName"`
	Status        string `json:"status"`
	OldStatus     string `json:"oldStatus,omitempty"`
}

// QueueInfo represents queue length info
type QueueInfo struct {
	QueueName string `json:"queueName"`
	Length    int64  `json:"length"`
}

// MysqlInfo represents MySQL health metrics
type MysqlInfo struct {
	Uptime                    string `json:"uptime"`
	ThreadsConnected          string `json:"threadsConnected"`
	AbortedConnects           string `json:"abortedConnects"`
	SlowQueries               string `json:"slowQueries"`
	CreatedTmpTables          string `json:"createdTmpTables"`
	CreatedTmpDiskTables      string `json:"createdTmpDiskTables"`
	TableLocksWaited          string `json:"tableLocksWaited"`
	ComRollback               string `json:"comRollback"`
}

// RedisInfo represents Redis health metrics
type RedisInfo struct {
	ProcessId                 string `json:"processId"`
	RedisVersion              string `json:"redisVersion"`
	GccVersion                string `json:"gccVersion"`
	UptimeInSeconds           string `json:"uptimeInSeconds"`
	UptimeInDays              string `json:"uptimeInDays"`
	ConnectedClients          string `json:"connectedClients"`
	TotalConnectionsReceived  string `json:"totalConnectionsReceived"`
	TotalCommandsProcessed    string `json:"totalCommandsProcessed"`
}

// HealthStatus represents overall health
type HealthStatus struct {
	Status       string            `json:"status"`
	Timestamp    string            `json:"timestamp"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

func newHealthStatus() HealthStatus {
	return HealthStatus{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
