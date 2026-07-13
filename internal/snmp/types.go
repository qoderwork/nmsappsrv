package snmp

// SnmpConnectionInfo holds SNMP target connection details
type SnmpConnectionInfo struct {
	IP                     string `json:"ip"`
	Port                   int    `json:"port"`
	Version                int    `json:"version"` // 0=v1, 1=v2c, 3=v3 (snmp4j 整数约定)
	Community              string `json:"community"`
	Username               string `json:"username"`
	Password               string `json:"password"`
	SecurityLevel          int    `json:"security_level"` // 1=noAuthNoPriv, 2=authNoPriv, 3=authPriv
	AuthenticationProtocol int    `json:"authentication_protocol"`
	PrivacyProtocol        int    `json:"privacy_protocol"`
	PrivacyPassphrase      string `json:"privacy_passphrase"`
	EngineId               string `json:"engine_id"`
	ContextName            string `json:"context_name"`
}

// SnmpParameter represents a single SNMP variable binding
type SnmpParameter struct {
	OID   string `json:"oid"`
	Type  string `json:"type"`  // "string", "int32", "uint32", "oid", "timeTicks", "ipv4", "int64"
	Value string `json:"value"`
}

// SnmpMessage represents a message in the Redis SNMP queue
type SnmpMessage struct {
	OperationType  int                `json:"operation_type"` // 0=TRAP, 1=GET, 2=SET, 3=WALK
	ConnectionInfo SnmpConnectionInfo `json:"connection_info"`
	Payload        []SnmpParameter    `json:"payload"`
}

const (
	OperationTrap = 0
	OperationGet  = 1
	OperationSet  = 2
	OperationWalk = 3
)

const (
	SnmpQueueName = "das_snmp_message_waiting_for_send"
)
