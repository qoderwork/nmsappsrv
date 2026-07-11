package initserver

import (
	"encoding/json"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/pkg/logger"
)

// Service contains init-server business logic.
type Service struct {
	repo *Repository
	mu   sync.Mutex
}

// NewService creates a new init-server service.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// GetConfig reads the init-server config from system_config.
func (s *Service) GetConfig() (*InitServerConfig, error) {
	return s.loadConfig()
}

// SaveConfig persists the init-server config to system_config.
// Per Java behavior, saving resets initServerEnable to "Disable".
func (s *Service) SaveConfig(cfg *InitServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.InitServerEnable = "Disable"
	return s.saveConfig(cfg)
}

// DealInitRequest processes a TR-069 request from a device during initial provisioning.
//
// Flow (mirrors Java InitServerManagementServiceImpl.dealInitRequest):
//  1. Empty SOAP → send SetParameterValues with init parameters to device
//  2. Inform → extract SN from DeviceId, respond with InformResponse
//  3. Other (TransferComplete, SPVResponse, etc.) → no-op (session complete)
//
// Returns the SOAP response string to send back to the device.
// The sn return value carries the extracted/confirmed device serial number.
func (s *Service) DealInitRequest(soapBody string, sn string) (responseSoap string, deviceSN string, err error) {
	if soapBody == "" {
		// Device connected with empty body — send init parameters
		response, err := s.sendInitSoap(sn)
		return response, sn, err
	}

	msgType := soap.DetectMessageType(soapBody)

	switch msgType {
	case soap.MsgInform:
		return s.handleInform(soapBody)
	default:
		// TransferComplete, SPVResponse, etc. — session complete, no response needed
		logger.Debugf("initserver: received %v from device %s, session complete", msgType, sn)
		return "", sn, nil
	}
}

// GenerateInitSoap builds a SetParameterValues SOAP message with all configured
// init parameters for the given device serial number.
// Exported for potential reuse by other modules (e.g., ZTP worker).
func (s *Service) GenerateInitSoap(sn string) (string, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return "", fmt.Errorf("load initserver config: %w", err)
	}
	params := buildParameterList(cfg, sn)
	headerId := soap.GenerateHeaderID()
	return soap.BuildSetParameterValues(headerId, params, ""), nil
}

// ---------- internal methods ----------

// sendInitSoap loads config and builds the SPV SOAP for device initialization.
func (s *Service) sendInitSoap(sn string) (string, error) {
	return s.GenerateInitSoap(sn)
}

// handleInform parses an Inform message, extracts the device SN from DeviceId,
// and returns an InformResponse SOAP message along with the extracted SN.
func (s *Service) handleInform(soapBody string) (string, string, error) {
	inf, err := soap.ParseInform(soapBody)
	if err != nil {
		return "", "", fmt.Errorf("parse Inform: %w", err)
	}

	sn := inf.DeviceId.SerialNumber
	logger.Infof("initserver: Inform from device SN=%s (manufacturer=%s, productClass=%s)",
		sn, inf.DeviceId.Manufacturer, inf.DeviceId.ProductClass)

	return soap.BuildInformResponse(inf.Header.ID), sn, nil
}

// buildParameterList converts InitServerConfig into TR-069 ParameterValueStruct entries.
// This is the core mapping between NMS config fields and CWMP parameter paths.
//
// Parameter types follow TR-069 conventions:
//   - xsd:string for text values
//   - xsd:boolean for enable/disable flags (mapped from "Enable"/"Disable" strings)
//   - xsd:unsignedInt for port numbers and numeric values
func buildParameterList(cfg *InitServerConfig, sn string) []soap.ParameterValueStruct {
	var params []soap.ParameterValueStruct

	// Helper closures for the three data types
	addStr := func(name, value string) {
		if value == "" {
			return
		}
		params = append(params, soap.ParameterValueStruct{
			Name: name, Value: value, Type: "xsd:string",
		})
	}
	addBool := func(name, value string) {
		if value == "" {
			return
		}
		params = append(params, soap.ParameterValueStruct{
			Name: name, Value: boolToSoap(value), Type: "xsd:boolean",
		})
	}
	addUint := func(name, value string) {
		if value == "" {
			return
		}
		params = append(params, soap.ParameterValueStruct{
			Name: name, Value: value, Type: "xsd:unsignedInt",
		})
	}

	// --- CA Server ---
	addStr("Device.CAServer.URL", cfg.CAUrl)
	addStr("Device.CAServer.Username", cfg.CAUsername)
	addStr("Device.CAServer.Password", cfg.CAPassword)

	// --- ACS ---
	addStr("Device.ManagementServer.URL", cfg.ACSURL)

	// --- IPsec General ---
	addBool("Device.IPsec.Enable", cfg.IPsecEnable)
	addStr("Device.IPsec.AuthenticationMethod", cfg.IPsecAuthenticationMethod)
	addStr("Device.IPsec.PreSharedKey", cfg.IPsecPreSharedKey)
	addStr("Device.IPsec.Certs", cfg.IPsecCerts)

	// --- IPsec Gateway ---
	addStr("Device.IPsec.Gateway.SecGWServer1", cfg.IPsecSecGWServer1)
	addStr("Device.IPsec.Gateway.SecGWServer2", cfg.IPsecSecGWServer2)
	addStr("Device.IPsec.Gateway.SecGWServer3", cfg.IPsecSecGWServer3)

	// --- IPsec Identity ---
	// LocalId is always set, using the device's serial number as value
	addStr("Device.IPsec.LocalId", sn)
	addStr("Device.IPsec.RemoteId", cfg.IPsecRemoteId)

	// --- IPsec Ports ---
	addUint("Device.IPsec.LocalPort", cfg.IPsecLocalPort)
	addUint("Device.IPsec.LocalNattPort", cfg.IPsecLocalNattPort)
	addUint("Device.IPsec.RemotePort", cfg.IPsecRemotePort)

	// --- IPsec EAP ---
	addStr("Device.IPsec.LocalEapId", cfg.IPsecLocalEapId)
	addStr("Device.IPsec.RemoteEapId", cfg.IPsecRemoteEapId)

	// --- IPsec Crypto ---
	addStr("Device.IPsec.OPC", cfg.IPsecOPC)
	addStr("Device.IPsec.K", cfg.IPsecK)
	addStr("Device.IPsec.EncryptionAlgorithms", cfg.IPsecEncryptionAlgorithms)
	addStr("Device.IPsec.IntegrityAlgorithms", cfg.IPsecIntegrityAlgorithms)
	addStr("Device.IPsec.DiffieHellmanGroupTransforms", cfg.IPsecDiffieHellmanGroupTransforms)

	// --- IPsec VIPS ---
	addBool("Device.IPsec.EnableVips", cfg.IPsecEnableVips)
	addBool("Device.IPsec.EnableVipsV6", cfg.IPsecEnableVipsV6)

	// --- IPsec DPD ---
	addUint("Device.IPsec.DpdDelay", cfg.IPsecDpdDelay)

	// --- IPsec ChildSA ---
	addUint("Device.IPsec.ChildSA.1.id", cfg.IPsecId)
	addStr("Device.IPsec.ChildSA.1.LocalTs", cfg.IPsecLocalTs)
	addStr("Device.IPsec.ChildSA.1.RemoteTs", cfg.IPsecRemoteTs)
	addStr("Device.IPsec.ChildSA.1.EncryptionAlgorithms", cfg.IPsecChildSAEncryptionAlgorithms)
	addStr("Device.IPsec.ChildSA.1.IntegrityAlgorithms", cfg.IPsecChildSAIntegrityAlgorithms)
	addStr("Device.IPsec.ChildSA.1.DiffieHellmanGroupTransforms", cfg.IPsecChildSADiffieHellmanGroupTransforms)

	return params
}

// boolToSoap converts "Enable"/"Disable" string to TR-069 boolean "true"/"false".
func boolToSoap(v string) string {
	if v == "Enable" {
		return "true"
	}
	return "false"
}

// ---------- repository helpers ----------

// loadConfig reads InitServerConfig from system_config table (key="initserver").
func (s *Service) loadConfig() (*InitServerConfig, error) {
	key := "initserver"
	sc, err := s.repo.FindByID(key)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &InitServerConfig{}, nil
		}
		return nil, err
	}
	if sc.Config == nil || *sc.Config == "" {
		return &InitServerConfig{}, nil
	}
	var cfg InitServerConfig
	if err := json.Unmarshal([]byte(*sc.Config), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// saveConfig writes InitServerConfig as JSON to system_config table.
// Creates the record if it doesn't exist, updates if it does.
func (s *Service) saveConfig(cfg *InitServerConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	val := string(data)
	key := "initserver"

	sc, err := s.repo.FindByID(key)
	if err == gorm.ErrRecordNotFound {
		return s.repo.Create(&SystemConfig{Id: key, Config: &val})
	}
	if err != nil {
		return err
	}
	sc.Config = &val
	return s.repo.Save(sc)
}
