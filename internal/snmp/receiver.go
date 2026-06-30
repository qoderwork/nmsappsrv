package snmp

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"gorm.io/gorm"
	"nmsappsrv/internal/device"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// haEnterpriseOID is the enterprise OID prefix for HA traps
const haEnterpriseOID = "1.3.6.1.4.1.1302.3.14.10"

// HA trap sub-OIDs
const (
	haNmsNameOID         = ".1.3.6.1.4.1.1302.3.14.10.1.5"
	haAdditionalInfoOID  = ".1.3.6.1.4.1.1302.3.14.10.1.6"
)

// TrapReceiver listens for incoming SNMP traps (e.g. HA traps)
type TrapReceiver struct {
	db       *gorm.DB
	port     int
	listener *gosnmp.TrapListener
}

// NewTrapReceiver creates a new TrapReceiver
func NewTrapReceiver(db *gorm.DB, port int) *TrapReceiver {
	if port <= 0 {
		port = 162
	}
	return &TrapReceiver{
		db:   db,
		port: port,
	}
}

// Start begins listening for SNMP traps in a background goroutine
func (r *TrapReceiver) Start() error {
	r.listener = gosnmp.NewTrapListener()
	r.listener.Params = &gosnmp.GoSNMP{
		Version: gosnmp.Version2c,
	}

	r.listener.OnNewTrap = r.handleTrap

	addr := fmt.Sprintf("0.0.0.0:%d", r.port)
	logger.Infof("SNMP trap receiver starting on %s", addr)

	utils.SafeGo("snmp-trap-receiver", func() {
		if err := r.listener.Listen(addr); err != nil {
			logger.Errorf("SNMP trap listener error: %v", err)
		}
	})

	return nil
}

// Stop closes the trap listener
func (r *TrapReceiver) Stop() {
	if r.listener != nil {
		r.listener.Close()
		logger.Info("SNMP trap receiver stopped")
	}
}

// handleTrap processes an incoming SNMP trap
func (r *TrapReceiver) handleTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	logger.Infof("SNMP trap received from %s", addr.String())

	// Parse variable bindings into SnmpParameters
	var params []SnmpParameter
	for _, pdu := range packet.Variables {
		param := pduToParameter(pdu)
		params = append(params, param)
		logger.Debugf("  Trap varbind: OID=%s Type=%s Value=%v", param.OID, param.Type, param.Value)
	}

	// Check if this is an HA trap
	if r.isHATrap(packet) {
		r.processHATrap(packet, params, addr)
	}

	// Process generic trap logging for all traps
	r.processGenericTrap(packet, params, addr)
}

// isHATrap checks whether the trap belongs to the HA enterprise OID
func (r *TrapReceiver) isHATrap(packet *gosnmp.SnmpPacket) bool {
	// For v2c/v3 traps, check the snmpTrapOID variable binding
	for _, pdu := range packet.Variables {
		if pdu.Name == ".1.3.6.1.6.3.1.1.4.1.0" || pdu.Name == "1.3.6.1.6.3.1.1.4.1.0" {
			val, ok := pdu.Value.(string)
			if ok && strings.HasPrefix(val, haEnterpriseOID) {
				return true
			}
		}
	}
	// For v1 traps, check the Enterprise field
	if packet.Version == gosnmp.Version1 {
		if strings.HasPrefix(packet.Enterprise, haEnterpriseOID) ||
			strings.HasPrefix(packet.Enterprise, "."+haEnterpriseOID) {
			return true
		}
	}
	return false
}

// processHATrap handles HA-specific trap processing - creates alarm records
func (r *TrapReceiver) processHATrap(packet *gosnmp.SnmpPacket, params []SnmpParameter, addr *net.UDPAddr) {
	logger.Infof("Processing HA trap from %s", addr.String())

	var nmsName string
	var additionalInfo string

	for _, pdu := range packet.Variables {
		oid := normalizeOID(pdu.Name)
		switch {
		case oid == haNmsNameOID:
			nmsName = pduValueToString(pdu)
		case oid == haAdditionalInfoOID:
			additionalInfo = pduValueToString(pdu)
		}
	}

	logger.Infof("HA trap: NMS=%s, AdditionalInfo=%s", nmsName, additionalInfo)

	// Create an alarm record in the database
	if r.db != nil {
		now := time.Now()
		alarmRecord := map[string]interface{}{
			"severity":              "informational",
			"alarm_identifier":      fmt.Sprintf("HA Trap from %s", nmsName),
			"probable_cause":        "haTrap",
			"alarm_source":          nmsName,
			"network_element":       nmsName,
			"event_type":            "communicationsAlarm",
			"alarm_status":          1,
			"alarm_type":            1,
			"event_time":            now,
			"specific_problem":      additionalInfo,
			"additional_information": additionalInfo,
			"create_time":           now,
			"update_time":           now,
		}

		if err := r.db.Table("alarm").Create(alarmRecord).Error; err != nil {
			logger.Errorf("Failed to create HA alarm record: %v", err)
		} else {
			logger.Infof("HA alarm record created for NMS: %s", nmsName)
		}
	}
}

// processGenericTrap logs every received trap into the snmp_trap_log table.
func (r *TrapReceiver) processGenericTrap(packet *gosnmp.SnmpPacket, params []SnmpParameter, addr *net.UDPAddr) {
	sourceIP := addr.IP.String()
	trapOID := extractTrapOID(packet)

	logger.Infof("Processing generic trap from %s, trapOID=%s", sourceIP, trapOID)

	// Marshal variable bindings to JSON
	varBindsJSON, err := json.Marshal(params)
	if err != nil {
		logger.Errorf("Failed to marshal trap varbinds: %v", err)
		varBindsJSON = []byte("[]")
	}

	// Look up the device by source IP in cpe_element table
	var elementId *int64
	if r.db != nil {
		var elem device.CpeElement
		if err := r.db.Table("cpe_element").
			Where("device_ip = ? AND deleted = ?", sourceIP, false).
			First(&elem).Error; err == nil {
			elementId = &elem.NeNeid
		} else {
			logger.Debugf("No device found for IP %s in cpe_element: %v", sourceIP, err)
		}
	}

	// Create the trap log entry
	trapLog := SnmpTrapLog{
		ElementId:   elementId,
		SourceIP:    sourceIP,
		TrapOID:     trapOID,
		VarBinds:    string(varBindsJSON),
		ReceiveTime: time.Now(),
	}

	if r.db != nil {
		if err := r.db.Create(&trapLog).Error; err != nil {
			logger.Errorf("Failed to create snmp_trap_log record: %v", err)
		} else {
			logger.Infof("snmp_trap_log created: id=%d, source=%s, trapOID=%s", trapLog.Id, sourceIP, trapOID)
		}
	}
}

// extractTrapOID returns the snmpTrapOID from the packet variables, or the enterprise OID for v1 traps.
func extractTrapOID(packet *gosnmp.SnmpPacket) string {
	// For v2c/v3, look for the snmpTrapOID variable binding
	for _, pdu := range packet.Variables {
		if pdu.Name == ".1.3.6.1.6.3.1.1.4.1.0" || pdu.Name == "1.3.6.1.6.3.1.1.4.1.0" {
			if val, ok := pdu.Value.(string); ok {
				return val
			}
		}
	}
	// For v1 traps, use the Enterprise field
	if packet.Version == gosnmp.Version1 {
		return packet.Enterprise
	}
	return "unknown"
}

// pduToParameter converts a gosnmp SnmpPDU to an SnmpParameter
func pduToParameter(pdu gosnmp.SnmpPDU) SnmpParameter {
	param := SnmpParameter{
		OID: pdu.Name,
	}

	switch pdu.Type {
	case gosnmp.OctetString:
		param.Type = "string"
		if bytes, ok := pdu.Value.([]byte); ok {
			param.Value = string(bytes)
		} else {
			param.Value = fmt.Sprintf("%v", pdu.Value)
		}
	case gosnmp.Integer:
		param.Type = "int32"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.Uinteger32:
		param.Type = "uint32"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	case gosnmp.Counter64:
		param.Type = "int64"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	case gosnmp.ObjectIdentifier:
		param.Type = "oid"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	case gosnmp.TimeTicks:
		param.Type = "timeTicks"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	case gosnmp.IPAddress:
		param.Type = "ipv4"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	default:
		param.Type = "string"
		param.Value = fmt.Sprintf("%v", pdu.Value)
	}

	return param
}

// pduValueToString extracts a string value from a PDU
func pduValueToString(pdu gosnmp.SnmpPDU) string {
	switch v := pdu.Value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// normalizeOID ensures the OID has a leading dot
func normalizeOID(oid string) string {
	if len(oid) > 0 && oid[0] != '.' {
		return "." + oid
	}
	return oid
}
