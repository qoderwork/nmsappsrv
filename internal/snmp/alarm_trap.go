package snmp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
)

// DefaultEnterpriseOID is the default enterprise OID for alarm traps
const DefaultEnterpriseOID = 31664

// OID prefix for alarm trap parameters
const alarmTrapOIDPrefix = ".1.3.6.1.4.1.%d.16"

// GetEnterpriseOID returns the enterprise OID from runtime config, falling back to the default.
func GetEnterpriseOID() uint32 {
	if configuredOID := config.GetSNMPEnterpriseOID(); configuredOID != "" {
		if oid, err := strconv.ParseUint(configuredOID, 10, 32); err == nil {
			return uint32(oid)
		}
	}
	return DefaultEnterpriseOID
}

// SendAlarmTrap builds and enqueues an SNMP trap for the given alarm
func SendAlarmTrap(db *gorm.DB, alarmId int64, alarmType int, severity string,
	probableCause string, eventType string, specificProblem string,
	networkElement string, alarmIdentifier string, alarmSource string,
	additionalInfo string, alarmIdStr string, handleSuggestion string,
	serialNumber string, deviceName string, femtoIps []string,
	nmsIp string, nmsHostname string, enterpriseOID int) error {

	if enterpriseOID <= 0 {
		enterpriseOID = int(GetEnterpriseOID())
	}

	oidPrefix := fmt.Sprintf(alarmTrapOIDPrefix, enterpriseOID)
	trapOID := fmt.Sprintf("1.3.6.1.4.1.%d.16", enterpriseOID)

	// Format event time as ISO 8601
	eventTime := time.Now().UTC().Format(time.RFC3339)

	// Format femto IPs as JSON array
	femtoIpsJSON := "[]"
	if len(femtoIps) > 0 {
		if data, err := json.Marshal(femtoIps); err == nil {
			femtoIpsJSON = string(data)
		}
	}

	// Prefix NMS hostname
	nmsHostnamePrefixed := nmsHostname
	if !strings.HasPrefix(nmsHostnamePrefixed, "NMS_") {
		nmsHostnamePrefixed = "NMS_" + nmsHostnamePrefixed
	}

	// Build the 18 SNMP parameters
	params := []SnmpParameter{
		{OID: oidPrefix + ".1", Type: "int32", Value: fmt.Sprintf("%d", alarmId)},
		{OID: oidPrefix + ".2", Type: "int32", Value: fmt.Sprintf("%d", alarmType)},
		{OID: oidPrefix + ".3", Type: "string", Value: alarmIdentifier},
		{OID: oidPrefix + ".4", Type: "string", Value: serialNumber},
		{OID: oidPrefix + ".5", Type: "string", Value: deviceName},
		{OID: oidPrefix + ".6", Type: "string", Value: probableCause},
		{OID: oidPrefix + ".7", Type: "string", Value: eventType},
		{OID: oidPrefix + ".8", Type: "string", Value: severity},
		{OID: oidPrefix + ".9", Type: "string", Value: eventTime},
		{OID: oidPrefix + ".10", Type: "string", Value: specificProblem},
		{OID: oidPrefix + ".11", Type: "string", Value: alarmSource},
		{OID: oidPrefix + ".12", Type: "string", Value: networkElement},
		{OID: oidPrefix + ".13", Type: "string", Value: alarmIdStr},
		{OID: oidPrefix + ".14", Type: "string", Value: additionalInfo},
		{OID: oidPrefix + ".15", Type: "string", Value: femtoIpsJSON},
		{OID: oidPrefix + ".16", Type: "string", Value: nmsIp},
		{OID: oidPrefix + ".17", Type: "string", Value: nmsHostnamePrefixed},
		{OID: oidPrefix + ".18", Type: "string", Value: handleSuggestion},
	}

	// Add the snmpTrapOID marker variable binding
	trapMarker := SnmpParameter{
		OID:   ".1.3.6.1.6.3.1.1.4.1.0",
		Type:  "oid",
		Value: trapOID,
	}
	params = append([]SnmpParameter{trapMarker}, params...)

	// Look up SNMP connection info from DB
	connInfos, err := getTrapTargets(db)
	if err != nil {
		logger.Errorf("Failed to get SNMP trap targets: %v", err)
		connInfos = []SnmpConnectionInfo{}
	}

	ctx := context.Background()
	for _, connInfo := range connInfos {
		msg := SnmpMessage{
			OperationType:  OperationTrap,
			ConnectionInfo: connInfo,
			Payload:        params,
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			logger.Errorf("Failed to marshal SNMP message: %v", err)
			continue
		}

		if err := redis.LPush(ctx, SnmpQueueName, string(msgJSON)); err != nil {
			logger.Errorf("Failed to enqueue SNMP message: %v", err)
			continue
		}
	}

	if len(connInfos) == 0 {
		logger.Warn("No SNMP trap targets configured, trap not enqueued")
	} else {
		logger.Infof("Enqueued %d SNMP trap message(s) for alarm %d", len(connInfos), alarmId)
	}

	return nil
}

// getTrapTargets retrieves configured SNMP trap targets from the database
func getTrapTargets(db *gorm.DB) ([]SnmpConnectionInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	type trapTarget struct {
		SnmpConnectionURL string `gorm:"column:snmp_connection_url"`
	}

	var targets []trapTarget
	err := db.Table("device").
		Select("snmp_connection_url").
		Where("snmp_connection_url IS NOT NULL AND snmp_connection_url != ''").
		Find(&targets).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query trap targets: %w", err)
	}

	var connInfos []SnmpConnectionInfo
	for _, t := range targets {
		if t.SnmpConnectionURL == "" {
			continue
		}
		connInfo, err := ParseConnectionURL(t.SnmpConnectionURL)
		if err != nil {
			logger.Errorf("Failed to parse SNMP connection URL %q: %v", t.SnmpConnectionURL, err)
			continue
		}
		connInfos = append(connInfos, *connInfo)
	}

	return connInfos, nil
}

// ParseConnectionURL parses an snmp:// URL into SnmpConnectionInfo
func ParseConnectionURL(rawURL string) (*SnmpConnectionInfo, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty SNMP connection URL")
	}

	parseURL := rawURL
	if strings.HasPrefix(parseURL, "snmp://") {
		parseURL = "http://" + parseURL[7:]
	} else {
		return nil, fmt.Errorf("invalid SNMP URL scheme: %s", rawURL)
	}

	u, err := url.Parse(parseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SNMP URL: %w", err)
	}

	info := &SnmpConnectionInfo{
		IP:   u.Hostname(),
		Port: 162,
	}

	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port in SNMP URL: %w", err)
		}
		info.Port = port
	}

	if u.User != nil {
		info.Version = 3
		info.Username = u.User.Username()
		info.Password, _ = u.User.Password()
	} else {
		info.Version = 1
	}

	q := u.Query()

	if community := q.Get("community"); community != "" {
		info.Community = community
	}

	if version := q.Get("version"); version != "" {
		switch strings.ToLower(version) {
		case "1", "v1":
			info.Version = 1
		case "2", "v2c", "2c":
			info.Version = 2
		case "3", "v3":
			info.Version = 3
		}
	}

	if secLevel := q.Get("securityLevel"); secLevel != "" {
		switch strings.ToLower(secLevel) {
		case "noauthnopriv":
			info.SecurityLevel = 1
		case "authnopriv":
			info.SecurityLevel = 2
		case "authpriv":
			info.SecurityLevel = 3
		}
	}

	if authProto := q.Get("authenticationProtocol"); authProto != "" {
		info.AuthenticationProtocol = parseAuthProtocol(authProto)
	}

	if privProto := q.Get("privacyProtocol"); privProto != "" {
		info.PrivacyProtocol = parsePrivProtocol(privProto)
	}

	if privPass := q.Get("privacyPassword"); privPass != "" {
		info.PrivacyPassphrase = privPass
	}

	if engineId := q.Get("engineId"); engineId != "" {
		info.EngineId = engineId
	}

	if ctxName := q.Get("contextName"); ctxName != "" {
		info.ContextName = ctxName
	}

	return info, nil
}

// parseAuthProtocol converts a string auth protocol name to the application code
func parseAuthProtocol(name string) int {
	switch strings.ToUpper(name) {
	case "SHA":
		return 1
	case "MD5":
		return 7
	case "HMAC128SHA224", "SHA224":
		return 3
	case "HMAC192SHA256", "SHA256":
		return 4
	case "HMAC256SHA384", "SHA384":
		return 5
	case "HMAC384SHA512", "SHA512":
		return 6
	default:
		return 0
	}
}

// parsePrivProtocol converts a string privacy protocol name to the application code
func parsePrivProtocol(name string) int {
	switch strings.ToUpper(name) {
	case "DES", "3DES":
		return 1
	case "AES", "AES128":
		return 2
	case "AES192":
		return 3
	case "AES256":
		return 4
	default:
		return 0
	}
}
