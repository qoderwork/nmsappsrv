package snmp

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/gosnmp/gosnmp"
	"nmsappsrv/pkg/logger"
)

// SendTrap sends an SNMP trap to the target
func SendTrap(connInfo SnmpConnectionInfo, params []SnmpParameter) error {
	snmpClient := &gosnmp.GoSNMP{
		Target:    connInfo.IP,
		Port:      uint16(connInfo.Port),
		Transport: "udp",
		Timeout:   5 * time.Second,
		Retries:   3,
	}

	switch connInfo.Version {
	case 0:
		snmpClient.Version = gosnmp.Version1
		snmpClient.Community = connInfo.Community
	case 1:
		snmpClient.Version = gosnmp.Version2c
		snmpClient.Community = connInfo.Community
	case 2:
		snmpClient.Version = gosnmp.Version2c
		snmpClient.Community = connInfo.Community
	case 3:
		snmpClient.Version = gosnmp.Version3
		snmpClient.SecurityModel = gosnmp.UserSecurityModel
		snmpClient.MsgFlags = getMsgFlags(connInfo.SecurityLevel)

		usmParams := &gosnmp.UsmSecurityParameters{
			UserName:                 connInfo.Username,
			AuthenticationPassphrase: connInfo.Password,
			PrivacyPassphrase:        connInfo.PrivacyPassphrase,
			AuthenticationProtocol:   getAuthProtocol(connInfo.AuthenticationProtocol),
			PrivacyProtocol:          getPrivProtocol(connInfo.PrivacyProtocol),
			AuthoritativeEngineID:    connInfo.EngineId,
		}
		snmpClient.SecurityParameters = usmParams
	default:
		return fmt.Errorf("unsupported SNMP version: %d", connInfo.Version)
	}

	if err := snmpClient.Connect(); err != nil {
		return fmt.Errorf("SNMP connect failed to %s:%d: %w", connInfo.IP, connInfo.Port, err)
	}
	defer snmpClient.Conn.Close()

	variables, err := buildTrapVariables(params)
	if err != nil {
		return fmt.Errorf("failed to build trap variables: %w", err)
	}

	trap := gosnmp.SnmpTrap{
		Variables: variables,
	}

	// For v1 traps, set enterprise-specific fields
	if connInfo.Version == 1 {
		trap.Enterprise = connInfo.EngineId
		trap.AgentAddress = connInfo.IP
		trap.GenericTrap = 6 // enterpriseSpecific
		trap.SpecificTrap = 1
	}

	_, err = snmpClient.SendTrap(trap)
	if err != nil {
		return fmt.Errorf("SNMP SendTrap failed to %s:%d: %w", connInfo.IP, connInfo.Port, err)
	}

	logger.Infof("SNMP trap sent successfully to %s:%d (v%d, %d variables)",
		connInfo.IP, connInfo.Port, connInfo.Version, len(variables))
	return nil
}

// buildTrapVariables converts SnmpParameters to gosnmp SnmpPDUs
func buildTrapVariables(params []SnmpParameter) ([]gosnmp.SnmpPDU, error) {
	var pdus []gosnmp.SnmpPDU

	for _, p := range params {
		pdu, err := convertToPDU(p)
		if err != nil {
			return nil, fmt.Errorf("failed to convert param OID=%s type=%s: %w", p.OID, p.Type, err)
		}
		pdus = append(pdus, pdu)
	}

	return pdus, nil
}

// convertToPDU converts a single SnmpParameter to a gosnmp SnmpPDU
func convertToPDU(p SnmpParameter) (gosnmp.SnmpPDU, error) {
	oid := p.OID
	// Ensure OID starts with a dot
	if len(oid) > 0 && oid[0] != '.' {
		oid = "." + oid
	}

	switch p.Type {
	case "string":
		// Try hex decoding first; if it fails, use raw string value
		var value interface{}
		if decoded, err := hex.DecodeString(p.Value); err == nil {
			value = decoded
		} else {
			value = p.Value
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.OctetString,
			Value: value,
		}, nil

	case "int32", "int8", "int16":
		val, err := strconv.ParseInt(p.Value, 10, 32)
		if err != nil {
			return gosnmp.SnmpPDU{}, fmt.Errorf("invalid int value %q: %w", p.Value, err)
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Integer,
			Value: int(val),
		}, nil

	case "int64":
		val, err := strconv.ParseInt(p.Value, 10, 64)
		if err != nil {
			return gosnmp.SnmpPDU{}, fmt.Errorf("invalid int64 value %q: %w", p.Value, err)
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Integer,
			Value: int(val),
		}, nil

	case "uint32", "uint8", "uint16":
		val, err := strconv.ParseUint(p.Value, 10, 32)
		if err != nil {
			return gosnmp.SnmpPDU{}, fmt.Errorf("invalid uint value %q: %w", p.Value, err)
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Counter32,
			Value: uint(val),
		}, nil

	case "gauge32":
		val, err := strconv.ParseUint(p.Value, 10, 32)
		if err != nil {
			return gosnmp.SnmpPDU{}, fmt.Errorf("invalid gauge32 value %q: %w", p.Value, err)
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Gauge32,
			Value: uint(val),
		}, nil

	case "counter64":
		val, err := strconv.ParseUint(p.Value, 10, 64)
		if err != nil {
			return gosnmp.SnmpPDU{}, fmt.Errorf("invalid counter64 value %q: %w", p.Value, err)
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Counter64,
			Value: val,
		}, nil

	case "oid":
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.ObjectIdentifier,
			Value: p.Value,
		}, nil

	case "timeTicks":
		val, err := strconv.ParseUint(p.Value, 10, 32)
		if err != nil {
			return gosnmp.SnmpPDU{}, fmt.Errorf("invalid timeTicks value %q: %w", p.Value, err)
		}
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.TimeTicks,
			Value: uint(val),
		}, nil

	case "ipv4":
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.IPAddress,
			Value: p.Value,
		}, nil

	default:
		// Default to OctetString for unknown types
		return gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.OctetString,
			Value: p.Value,
		}, nil
	}
}

// getAuthProtocol maps the application auth protocol code to gosnmp constant
func getAuthProtocol(code int) gosnmp.SnmpV3AuthProtocol {
	switch code {
	case 1:
		return gosnmp.SHA
	case 7:
		return gosnmp.MD5
	case 3:
		return gosnmp.SHA224
	case 4:
		return gosnmp.SHA256
	case 5:
		return gosnmp.SHA384
	case 6:
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

// getPrivProtocol maps the application privacy protocol code to gosnmp constant
func getPrivProtocol(code int) gosnmp.SnmpV3PrivProtocol {
	switch code {
	case 1:
		return gosnmp.DES
	case 2:
		return gosnmp.AES
	case 3:
		return gosnmp.AES192
	case 4:
		return gosnmp.AES256
	case 5:
		return gosnmp.AES192C
	case 6:
		return gosnmp.AES256C
	default:
		return gosnmp.NoPriv
	}
}

// getMsgFlags maps the security level to gosnmp MsgFlags
func getMsgFlags(level int) gosnmp.SnmpV3MsgFlags {
	switch level {
	case 1:
		return gosnmp.NoAuthNoPriv
	case 2:
		return gosnmp.AuthNoPriv
	case 3:
		return gosnmp.AuthPriv
	default:
		return gosnmp.NoAuthNoPriv
	}
}
