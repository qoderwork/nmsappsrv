package topology

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"nmsappsrv/internal/device"
	"nmsappsrv/internal/tr069"
	"nmsappsrv/internal/tr069/soap"
	"nmsappsrv/internal/upgrade"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"

	"gorm.io/gorm"
)

// Service contains the business logic for topology management.
type Service struct {
	repo      *Repository
	opSender  *tr069.OperationSender
}

// NewService creates a new Service.
func NewService(db *gorm.DB, opSender *tr069.OperationSender) Service {
	return Service{repo: NewRepository(db), opSender: opSender}
}

// LteTopology builds the LTE topology tree for a given BBU element.
func (s *Service) LteTopology(elementId int64) (*LteTopologyResponse, error) {
	elem, err := s.repo.FindElementById(elementId)
	if err != nil {
		return nil, apperror.New("DEVICE_NOT_FOUND", 404, "The device has been deleted")
	}

	resp := &LteTopologyResponse{
		BBU: &TopologyBBU{
			DeviceName:   elem.DeviceName,
			SerialNumber: elem.SerialNumber,
			Ports:        []TopologyBBUPort{},
		},
	}

	allRouteParamRU, _ := s.repo.FindLeafParamsByElementIdAndNameLike(elementId,
		"Device.Services.X_BBU_RU%NODE.%.RoutingInformation")
	allRouteParamERU, _ := s.repo.FindLeafParamsByElementIdAndNameLike(elementId,
		"Device.Services.X_BBU_ERU%NODE.%.RoutingInformation")
	allRouteParam := append(allRouteParamRU, allRouteParamERU...)

	firstLevelRegex := regexp.MustCompile(`^[0-9]+\.0\.0\.0\.0\.0\.0\.0$`)
	for _, p := range allRouteParam {
		pv := strVal(p.ParamValue)
		if pv == "" || !firstLevelRegex.MatchString(pv) {
			continue
		}
		port := pv[:strings.Index(pv, ".")]
		prefix := paramPrefix(p.ParamName)
		paramNames := []string{
			prefix + ".RRUSoftwareVersion",
			prefix + ".RRUType",
			prefix + ".ProductionSerialNumber",
			prefix + ".RRUIP",
			prefix + ".ConnectStatus",
			prefix + ".RoutingInformation",
		}
		params, _ := s.repo.FindParamsByElementIdAndNameIn(elementId, paramNames)
		nvMap := buildNameValueMap(params)
		portIdx := 0
		fmt.Sscanf(port, "%d", &portIdx)
		portVO := TopologyBBUPort{
			PortIndex:        portIdx,
			DeviceType:       "RU",
			Type:             nvMap[prefix+".RRUType"],
			SerialNumber:     nvMap[prefix+".ProductionSerialNumber"],
			IP:               nvMap[prefix+".RRUIP"],
			SoftwareVersion:  nvMap[prefix+".RRUSoftwareVersion"],
			ConnectStatus:    connStatus(nvMap[prefix+".ConnectStatus"]),
			Route:            nvMap[prefix+".RoutingInformation"],
			NextLevelDevices: s.dealCascadeRU(allRouteParam, port, elementId, 0),
		}
		resp.BBU.Ports = append(resp.BBU.Ports, portVO)
	}
	return resp, nil
}

func (s *Service) dealCascadeRU(allRoute []device.ElementBasicInfoParameter, port string, eid int64, level int) []TopologyDevice {
	target := fmt.Sprintf("%s.%d.0.0.0.0.0.0", port, level+1)
	var next *device.ElementBasicInfoParameter
	for i := range allRoute {
		if strVal(allRoute[i].ParamValue) == target {
			next = &allRoute[i]
			break
		}
	}
	if next == nil {
		return []TopologyDevice{}
	}
	prefix := paramPrefix(next.ParamName)
	names := []string{
		prefix + ".RRUSoftwareVersion", prefix + ".RRUType",
		prefix + ".ProductionSerialNumber", prefix + ".RRUIP",
		prefix + ".ConnectStatus", prefix + ".RoutingInformation",
	}
	params, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
	nv := buildNameValueMap(params)
	return []TopologyDevice{{
		DeviceType: "RU", Type: nv[prefix+".RRUType"],
		SerialNumber: nv[prefix+".ProductionSerialNumber"],
		IP: nv[prefix+".RRUIP"], SoftwareVersion: nv[prefix+".RRUSoftwareVersion"],
		ConnectStatus: connStatus(nv[prefix+".ConnectStatus"]),
		Route: nv[prefix+".RoutingInformation"],
		NextLevelDevices: s.dealCascadeRU(allRoute, port, eid, level+1),
	}}
}

// NrTopology builds the NR topology tree for a given gNB element.
func (s *Service) NrTopology(elementId int64) (*LteTopologyResponse, error) {
	elem, err := s.repo.FindElementById(elementId)
	if err != nil {
		return nil, apperror.New("DEVICE_NOT_FOUND", 404, "The device has been deleted")
	}
	resp := &LteTopologyResponse{
		BBU: &TopologyBBU{
			DeviceName: elem.DeviceName, SerialNumber: elem.SerialNumber,
			Ports: []TopologyBBUPort{},
		},
	}
	nwParams, _ := s.repo.FindParamsByElementIdAndNameLike(elementId, "Device.X_BBU_System.OptPortMapping.OPT%LinkType")
	portLink := map[int]string{}
	for _, p := range nwParams {
		pn := strVal(p.ParamName)
		pv := strVal(p.ParamValue)
		switch {
		case strings.HasSuffix(pn, "OPT1LinkType"):
			portLink[1] = pv
		case strings.HasSuffix(pn, "OPT2LinkType"):
			portLink[2] = pv
		case strings.HasSuffix(pn, "OPT3LinkType"):
			portLink[3] = pv
		case strings.HasSuffix(pn, "OPT4LinkType"):
			portLink[4] = pv
		}
	}
	if portLink[1] == "" || portLink[2] == "" || portLink[3] == "" || portLink[4] == "" {
		return nil, apperror.New("TOPOLOGY_DATA_ABNORMAL", 400, "The topology data is abnormal. Please reload it")
	}
	allRU, _ := s.repo.FindLeafParamsByElementIdAndNameLike(elementId, "Device.Services.X_BBU_RU%NODE.%.RoutingInformation")
	allEU, _ := s.repo.FindParamsByElementIdAndNameLike(elementId, "Device.Services.X_BBU_EU.%.RoutingInformation")
	for i := 1; i <= 4; i++ {
		switch portLink[i] {
		case "RRU":
			s.nrRRU(elementId, &resp.BBU.Ports, allRU, i)
		case "EU":
			s.nrEU(elementId, &resp.BBU.Ports, allEU, i)
		case "AU":
			s.nrAU(elementId, &resp.BBU.Ports, i)
		}
	}
	return resp, nil
}

func (s *Service) nrRRU(eid int64, ports *[]TopologyBBUPort, allRU []device.ElementBasicInfoParameter, idx int) {
	target := fmt.Sprintf("%d.0.0.0.0.0.0.0", idx)
	rp, err := s.repo.FindLeafParamByElementIdAndNameLikeAndValue(eid, "Device.Services.X_BBU_RU%NODE.%.RoutingInformation", target)
	if err != nil {
		return
	}
	prefix := paramPrefix(rp.ParamName)
	names := []string{prefix + ".RRUSoftwareVersion", prefix + ".RRUType", prefix + ".ProductionSerialNumber", prefix + ".RRUIP", prefix + ".ConnectStatus", prefix + ".RoutingInformation"}
	params, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
	nv := buildNameValueMap(params)
	pvo := TopologyBBUPort{
		PortIndex: idx, DeviceType: "RU",
		SerialNumber: nv[prefix+".ProductionSerialNumber"], IP: nv[prefix+".RRUIP"],
		SoftwareVersion: nv[prefix+".RRUSoftwareVersion"], ConnectStatus: connStatus(nv[prefix+".ConnectStatus"]),
		Route: nv[prefix+".RoutingInformation"], NextLevelDevices: []TopologyDevice{},
	}
	re := regexp.MustCompile(`X_BBU_RU(\d+)NODE`)
	if m := re.FindStringSubmatch(strVal(rp.ParamName)); len(m) > 1 {
		pvo.Type = "RU" + m[1]
	}
	*ports = append(*ports, pvo)
}

func (s *Service) nrEU(eid int64, ports *[]TopologyBBUPort, allEU []device.ElementBasicInfoParameter, idx int) {
	target := fmt.Sprintf("%d.0.0.0.0.0.0.0", idx)
	var euInst string
	for _, p := range allEU {
		if strVal(p.ParamValue) == target {
			parts := strings.Split(strVal(p.ParamName), ".")
			if len(parts) > 3 {
				euInst = parts[3]
			}
		}
	}
	if euInst == "" {
		return
	}
	pre := "Device.Services.X_BBU_EU." + euInst + "."
	names := []string{pre + "SoftwareVersion", pre + "Status", pre + "SerialNumber", pre + "IP", pre + "EUName", pre + "DevType"}
	params, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
	nv := buildNameValueMap(params)
	pvo := TopologyBBUPort{
		PortIndex: idx, DeviceType: "EU", Route: target,
		IP: nv[pre+"IP"], SerialNumber: nv[pre+"SerialNumber"],
		ConnectStatus: connStatus(nv[pre+"Status"]), Type: nv[pre+"DevType"],
		SoftwareVersion: nv[pre+"SoftwareVersion"],
	}
	if pvo.SerialNumber == "" {
		return
	}
	ruOnEU, _ := s.repo.FindParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_EU.%.RRU.%.RoutingInformation")
	firstGrade := []TopologyDevice{}
	s.ruOnEU(eid, ruOnEU, euInst, &firstGrade, idx)
	s.euCascade(eid, allEU, &pvo, &firstGrade)
	pvo.NextLevelDevices = firstGrade
	*ports = append(*ports, pvo)
}

func (s *Service) ruOnEU(eid int64, ruParams []device.ElementBasicInfoParameter, euInst string, devs *[]TopologyDevice, idx int) {
	grex := fmt.Sprintf(`^%d\.[0-9]+\.0\.0\.0\.0\.0\.0$`, idx)
	re := regexp.MustCompile(grex)
	euRe := regexp.MustCompile(`^Device\.Services\.X_BBU_EU\.[0-9]+\.RRU\.[0-9]+\.RoutingInformation$`)
	for _, p := range ruParams {
		pv := strVal(p.ParamValue)
		pn := strVal(p.ParamName)
		if pv == "" || !re.MatchString(pv) || !euRe.MatchString(pn) || pv == fmt.Sprintf("%d.0.0.0.0.0.0.0", idx) {
			continue
		}
		parts := strings.Split(pv, ".")
		if len(parts) < 2 {
			continue
		}
		ruPort := parts[1]
		np := strings.Split(pn, ".")
		if len(np) < 6 {
			continue
		}
		ruInst := np[5]
		pre := "Device.Services.X_BBU_EU." + euInst + ".RRU." + ruInst + "."
		names := []string{pre + "SerialNumber", pre + "Status", pre + "SoftwareVersion", pre + "DevType"}
		params, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
		nv := buildNameValueMap(params)
		pi := 0
		fmt.Sscanf(ruPort, "%d", &pi)
		td := TopologyDevice{
			DeviceType: "RU", PortIndex: pi, Route: pv,
			Type: nv[pre+"DevType"], ConnectStatus: connStatus(nv[pre+"Status"]),
			SerialNumber: nv[pre+"SerialNumber"], SoftwareVersion: nv[pre+"SoftwareVersion"],
			NextLevelDevices: []TopologyDevice{},
		}
		if td.SerialNumber != "" {
			*devs = append(*devs, td)
		}
	}
}

func (s *Service) euCascade(eid int64, allEU []device.ElementBasicInfoParameter, pvo *TopologyBBUPort, devs *[]TopologyDevice) {
	cascRe := regexp.MustCompile(`^Device\.Services\.X_BBU_EU\.[0-9]+\.RoutingInformation$`)
	routeRe := regexp.MustCompile(fmt.Sprintf(`^%d\.[0-9]+\.0\.0\.0\.0\.0\.0$`, pvo.PortIndex))
	for _, p := range allEU {
		pv := strVal(p.ParamValue)
		pn := strVal(p.ParamName)
		if pv == "" || !cascRe.MatchString(pn) || !routeRe.MatchString(pv) || pv == pvo.Route {
			continue
		}
		np := strings.Split(pn, ".")
		if len(np) < 4 {
			continue
		}
		cInst := np[3]
		rp := strings.Split(pv, ".")
		cPort := 0
		for i, r := range rp {
			if r == "0" && i > 0 {
				fmt.Sscanf(rp[i-1], "%d", &cPort)
				break
			}
		}
		pre := "Device.Services.X_BBU_EU." + cInst + "."
		names := []string{pre + "SoftwareVersion", pre + "Status", pre + "SerialNumber", pre + "IP", pre + "EUName", pre + "DevType"}
		params, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
		nv := buildNameValueMap(params)
		ceu := TopologyDevice{
			DeviceType: "EU", Route: pv, PortIndex: cPort,
			SerialNumber: nv[pre+"SerialNumber"], ConnectStatus: connStatus(nv[pre+"Status"]),
			IP: nv[pre+"IP"], Type: nv[pre+"DevType"], SoftwareVersion: nv[pre+"SoftwareVersion"],
			NextLevelDevices: []TopologyDevice{},
		}
		if ceu.SerialNumber == "" {
			continue
		}
		ruOnEU, _ := s.repo.FindParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_EU.%.RRU.%.RoutingInformation")
		s.ruOnEU(eid, ruOnEU, cInst, &ceu.NextLevelDevices, cPort)
		*devs = append(*devs, ceu)
	}
}

func (s *Service) nrAU(eid int64, ports *[]TopologyBBUPort, idx int) {
	params, _ := s.repo.FindParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_AU%.%.DevRouteAddr")
	auRe := regexp.MustCompile(`^Device\.Services\.X_BBU_AU300\.[0-9]+\.DevRouteAddr$`)
	for _, p := range params {
		pv := strVal(p.ParamValue)
		pn := strVal(p.ParamName)
		if pv == "" || !auRe.MatchString(pn) || !strings.HasPrefix(pv, fmt.Sprintf("%d", idx)) {
			continue
		}
		np := strings.Split(pn, ".")
		if len(np) < 4 {
			continue
		}
		inst := np[3]
		pre := "Device.Services.X_BBU_AU300." + inst + "."
		names := []string{pre + "SerialNumber", pre + "DevType", pre + "SoftwareVersion", pre + "Status"}
		ps, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
		nv := buildNameValueMap(ps)
		pi := 0
		rp := strings.Split(pv, ".")
		if len(rp) > 0 {
			fmt.Sscanf(rp[0], "%d", &pi)
		}
		vo := TopologyBBUPort{
			PortIndex: pi, DeviceType: "AU", Route: pv,
			Type: nv[pre+"DevType"], SerialNumber: nv[pre+"SerialNumber"],
			ConnectStatus: connStatus(nv[pre+"Status"]), SoftwareVersion: nv[pre+"SoftwareVersion"],
			NextLevelDevices: []TopologyDevice{},
		}
		if vo.SerialNumber == "" {
			continue
		}
		s.euOnAU(eid, &vo)
		s.ruOnAU(eid, &vo)
		*ports = append(*ports, vo)
	}
}

func (s *Service) euOnAU(eid int64, au *TopologyBBUPort) {
	params, _ := s.repo.FindParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_AU300.%.EU300.%.DevRouteAddr")
	s.doEUAU(eid, au.Route, func(td TopologyDevice) {
		au.NextLevelDevices = append(au.NextLevelDevices, td)
	}, params)
}

func (s *Service) doEUAU(eid int64, parentRoute string, addChild func(TopologyDevice), params []device.ElementBasicInfoParameter) {
	reg := nextDevReg(parentRoute)
	re := regexp.MustCompile(reg)
	for _, p := range params {
		pv := strVal(p.ParamValue)
		if pv == "" || !re.MatchString(pv) {
			continue
		}
		prefix := paramPrefix(p.ParamName)
		rp := strings.Split(pv, ".")
		pi := 0
		if len(rp) > 1 {
			fmt.Sscanf(rp[1], "%d", &pi)
		}
		names := []string{prefix + ".SerialNumber", prefix + ".Status", prefix + ".DevType", prefix + ".SoftwareVersion"}
		ps, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
		nv := buildNameValueMap(ps)
		eu := TopologyDevice{
			DeviceType: "EU", SerialNumber: nv[prefix+".SerialNumber"],
			ConnectStatus: connStatus(nv[prefix+".Status"]), Type: nv[prefix+".DevType"],
			SoftwareVersion: nv[prefix+".SoftwareVersion"], Route: pv, PortIndex: pi,
			NextLevelDevices: []TopologyDevice{},
		}
		if eu.SerialNumber == "" {
			continue
		}
		addChild(eu)
		// Recurse for cascaded EUs
		s.doEUAU(eid, pv, func(td TopologyDevice) {
			eu.NextLevelDevices = append(eu.NextLevelDevices, td)
		}, params)
		// RUs under this EU in AU mode
		ruParams, _ := s.repo.FindParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_AU300.%.RU8250.%.DevRouteAddr")
		s.doRU(eid, pv, func(td TopologyDevice) {
			eu.NextLevelDevices = append(eu.NextLevelDevices, td)
		}, ruParams)
	}
}

func (s *Service) ruOnAU(eid int64, au *TopologyBBUPort) {
	ruParams, _ := s.repo.FindParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_AU300.%.RU8250.%.DevRouteAddr")
	s.doRU(eid, au.Route, func(td TopologyDevice) {
		au.NextLevelDevices = append(au.NextLevelDevices, td)
	}, ruParams)
}

func (s *Service) doRU(eid int64, parentRoute string, addChild func(TopologyDevice), ruParams []device.ElementBasicInfoParameter) {
	reg := nextDevReg(parentRoute)
	re := regexp.MustCompile(reg)
	for _, p := range ruParams {
		pv := strVal(p.ParamValue)
		if pv == "" || !re.MatchString(pv) {
			continue
		}
		prefix := paramPrefix(p.ParamName)
		pi := 0
		fmt.Sscanf(extractPort(pv), "%d", &pi)
		names := []string{prefix + ".SerialNumber", prefix + ".DevType", prefix + ".SoftwareVersion", prefix + ".Status"}
		ps, _ := s.repo.FindParamsByElementIdAndNameIn(eid, names)
		nv := buildNameValueMap(ps)
		ru := TopologyDevice{
			DeviceType: "RU", Route: pv, PortIndex: pi,
			SoftwareVersion: nv[prefix+".SoftwareVersion"], SerialNumber: nv[prefix+".SerialNumber"],
			Type: nv[prefix+".DevType"], ConnectStatus: connStatus(nv[prefix+".Status"]),
			NextLevelDevices: []TopologyDevice{},
		}
		if ru.SerialNumber == "" {
			continue
		}
		addChild(ru)
	}
}

// BatchUpgradeEUAndRU creates a batch upgrade task for EU and RU devices.
// It saves a local log and sends the BatchUpgrade SOAP to the BBU device,
// mirroring Java BatchUpgradeEUAndRU (which also persists and dispatches).
func (s *Service) BatchUpgradeEUAndRU(req *BatchUpgradeRequest, tenancyId int, username string) error {
	if req.ElementId == 0 || len(req.Devices) == 0 {
		return apperror.ErrInvalidInput
	}
	for _, d := range req.Devices {
		if d.Route == "" || d.UpgradeFileId == 0 {
			return apperror.ErrInvalidInput
		}
	}
	elem, err := s.repo.FindElementById(req.ElementId)
	if err != nil {
		return apperror.New("DEVICE_NOT_FOUND", 404, "The device has been deleted")
	}

	oldVersions := []EUAndRUUpgradeDTO{}
	if strVal(elem.Generation) == "LTE" {
		allRoute := s.allRoutParams(req.ElementId)
		allVer := s.allVerParams(req.ElementId)
		allSN := s.allSNParams(req.ElementId)
		r2n := buildValToNameMap(allRoute)
		n2v := buildNameValueMap(allVer)
		n2s := buildNameValueMap(allSN)
		for _, d := range req.Devices {
			dto := EUAndRUUpgradeDTO{UpgradeFileId: d.UpgradeFileId, Route: d.Route}
			rpn := r2n[d.Route]
			if rpn != "" {
				pfx := paramPrefix(&rpn)
				dto.Version = n2v[pfx+".RRUSoftwareVersion"]
				dto.SerialNumber = n2s[pfx+".ProductionSerialNumber"]
			}
			oldVersions = append(oldVersions, dto)
		}
	} else {
		r1, _ := s.repo.FindLeafParamsByElementIdAndNameLike(req.ElementId, "Device.Services.X_BBU%.RoutingInformation")
		r2, _ := s.repo.FindLeafParamsByElementIdAndNameLike(req.ElementId, "Device.Services.X_BBU_AU300.%.DevRouteAddr")
		allRoute := append(r1, r2...)
		allVer, _ := s.repo.FindLeafParamsByElementIdAndNameLike(req.ElementId, "Device.Services.X_BBU%SoftwareVersion")
		allSN, _ := s.repo.FindLeafParamsByElementIdAndNameLike(req.ElementId, "Device.Services.X_BBU%SerialNumber")
		v2n := buildValToNameMap(allRoute)
		n2v := buildNameValueMap(allVer)
		n2s := buildNameValueMap(allSN)
		for _, d := range req.Devices {
			pn := v2n[d.Route]
			dto := EUAndRUUpgradeDTO{UpgradeFileId: d.UpgradeFileId, Route: d.Route}
			if pn != "" {
				pfx := paramPrefix(&pn)
				var vn, sn string
				if strings.Contains(pn, "EU") || strings.Contains(pn, "AU300") {
					vn = pfx + ".SoftwareVersion"
					sn = pfx + ".SerialNumber"
				} else {
					vn = pfx + ".RRUSoftwareVersion"
					sn = pfx + ".ProductionSerialNumber"
				}
				dto.SerialNumber = n2s[sn]
				dto.Version = n2v[vn]
			}
			oldVersions = append(oldVersions, dto)
		}
	}

	ovJSON, _ := json.Marshal(oldVersions)
	now := time.Now()
	log := upgrade.EUAndRUBatchUpgradeLog{
		TenancyId: &tenancyId, ElementId: &req.ElementId,
		User: &username, OperationTime: &now,
		OriginalVersion: strPtr(string(ovJSON)),
	}
	if err := s.repo.SaveBatchUpgradeLog(&log); err != nil {
		logger.Errorf("failed to save batch upgrade log: %v", err)
		return apperror.Wrap(err, "SAVE_FAILED", 500, "failed to save batch upgrade log")
	}

	if s.opSender == nil {
		logger.Warnf("BatchUpgradeEUAndRU: opSender not configured, cannot dispatch SOAP")
		return nil
	}

	upgradeDevices := make([]soap.UpgradeDeviceStruct, 0, len(req.Devices))
	for _, d := range req.Devices {
		var uf upgrade.UpgradeFile
		if err := s.repo.db.Table("upgrade_file").
			Where("id = ? AND deleted = ?", d.UpgradeFileId, false).
			First(&uf).Error; err != nil {
			logger.Warnf("BatchUpgradeEUAndRU: upgrade file %d not found", d.UpgradeFileId)
			continue
		}
		upgradeDevices = append(upgradeDevices, soap.UpgradeDeviceStruct{
			DeviceRouteList: []string{d.Route},
			URL:             strVal(uf.FilePath),
			FileSize:        int64(strValP(uf.FileSize, 0)),
			TargetFileName:  strVal(uf.FileName),
		})
	}

	if len(upgradeDevices) == 0 {
		logger.Warnf("BatchUpgradeEUAndRU: no valid upgrade files found")
		return nil
	}

	commandKey := fmt.Sprintf("X_%d", log.Id)
	batch := &soap.BatchUpgrade{
		CommandKey:        commandKey,
		UpgradeDeviceList: upgradeDevices,
	}

	operationId := fmt.Sprintf("batch_upgrade_%d", log.Id)
	if err := s.opSender.SendBatchUpgrade(strVal(elem.SerialNumber), batch, operationId); err != nil {
		logger.Errorf("BatchUpgradeEUAndRU: failed to send SOAP to %s: %v", strVal(elem.SerialNumber), err)
		return apperror.Wrap(err, "SEND_FAILED", 500, "failed to send batch upgrade command")
	}

	return nil
}

// ListBatchUpgradeLog returns paginated batch upgrade logs.
func (s *Service) ListBatchUpgradeLog(q *ListBatchUpgradeLogQuery) ([]ListBatchUpgradeLogVO, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	offset := (q.Page - 1) * q.PageSize
	logs, total, err := s.repo.FindBatchUpgradeLogs(q, offset, q.PageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]ListBatchUpgradeLogVO, 0, len(logs))
	for _, lg := range logs {
		vo := ListBatchUpgradeLogVO{
			OperationTime: lg.OperationTime, DownloadedTime: lg.DownloadedTime,
			UpgradedTime: lg.UpgradedTime, Result: lg.Result,
			OperationUser: strVal(lg.User), FaultInfo: strVal(lg.FaultInfo),
		}
		vcl := []VersionChange{}
		if strVal(lg.OriginalVersion) != "" {
			var origDTOs []EUAndRUUpgradeDTO
			if json.Unmarshal([]byte(*lg.OriginalVersion), &origDTOs) == nil {
				sn2v := map[string]string{}
				sn2f := map[string]string{}
				if strVal(lg.UpgradedVersion) != "" {
					var upDTOs []EUAndRUUpgradeDTO
					if json.Unmarshal([]byte(*lg.UpgradedVersion), &upDTOs) == nil {
						for _, u := range upDTOs {
							if u.Version != "" {
								sn2v[u.SerialNumber] = u.Version
							}
							if u.FaultInfo != "" {
								sn2f[u.SerialNumber] = u.FaultInfo
							}
						}
					}
				}
				fidSet := map[int]bool{}
				for _, dto := range origDTOs {
					fidSet[dto.UpgradeFileId] = true
				}
				fids := make([]int, 0, len(fidSet))
				for id := range fidSet {
					fids = append(fids, id)
				}
				files, _ := s.repo.FindUpgradeFilesByIds(fids)
				id2fn := map[int]string{}
				for _, f := range files {
					if f.Id != 0 && f.OriginalFileName != nil {
						id2fn[f.Id] = *f.OriginalFileName
					}
				}
				for _, dto := range origDTOs {
					vcl = append(vcl, VersionChange{
						SerialNumber: dto.SerialNumber, OriginalVersion: dto.Version,
						UpgradedVersion: sn2v[dto.SerialNumber], UpgradeFileName: id2fn[dto.UpgradeFileId],
						FaultInfo: sn2f[dto.SerialNumber],
					})
				}
			}
		}
		vo.VersionInfo = vcl
		vos = append(vos, vo)
	}
	return vos, total, nil
}

// ReloadLTETopo clears cached topology parameters for re-collection.
func (s *Service) ReloadLTETopo(elementId int64) error {
	if elementId == 0 {
		return apperror.ErrInvalidInput
	}
	ruP, _ := s.repo.FindParamsByElementIdAndNameLike(elementId, "Device.Services.X_BBU_RU%NODE.%")
	if len(ruP) > 0 {
		s.repo.DeleteParamsByIds(elementId, paramIds(ruP))
	}
	opP, _ := s.repo.FindParamsByElementIdAndNameLike(elementId, "Device.X_BBU_System.OptPortMapping.OPT%LinkType")
	if len(opP) > 0 {
		s.repo.DeleteParamsByIds(elementId, paramIds(opP))
	}
	euP, _ := s.repo.FindParamsByElementIdAndNameLike(elementId, "Device.Services.X_BBU_EU.%")
	if len(euP) > 0 {
		s.repo.DeleteParamsByIds(elementId, paramIds(euP))
	}
	return nil
}

func (s *Service) allRoutParams(eid int64) []device.ElementBasicInfoParameter {
	r, _ := s.repo.FindLeafParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_RU%NODE.%.RoutingInformation")
	e, _ := s.repo.FindLeafParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_ERU%NODE.%.RoutingInformation")
	return append(r, e...)
}
func (s *Service) allVerParams(eid int64) []device.ElementBasicInfoParameter {
	r, _ := s.repo.FindLeafParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_RU%NODE.%.RRUSoftwareVersion")
	e, _ := s.repo.FindLeafParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_ERU%NODE.%.RRUSoftwareVersion")
	return append(r, e...)
}
func (s *Service) allSNParams(eid int64) []device.ElementBasicInfoParameter {
	r, _ := s.repo.FindLeafParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_RU%NODE.%.ProductionSerialNumber")
	e, _ := s.repo.FindLeafParamsByElementIdAndNameLike(eid, "Device.Services.X_BBU_ERU%NODE.%.ProductionSerialNumber")
	return append(r, e...)
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtr(s string) *string {
	return &s
}

func strValP(p *int64, def int64) int64 {
	if p == nil {
		return def
	}
	return *p
}

// paramPrefix returns param_name up to the last ".".
func paramPrefix(pn *string) string {
	if pn == nil {
		return ""
	}
	idx := strings.LastIndex(*pn, ".")
	if idx < 0 {
		return *pn
	}
	return (*pn)[:idx]
}

func buildNameValueMap(params []device.ElementBasicInfoParameter) map[string]string {
	m := make(map[string]string, len(params))
	for _, p := range params {
		pn := strVal(p.ParamName)
		pv := strVal(p.ParamValue)
		if pn != "" && pv != "" {
			m[pn] = pv
		}
	}
	return m
}

func buildValToNameMap(params []device.ElementBasicInfoParameter) map[string]string {
	m := make(map[string]string, len(params))
	for _, p := range params {
		pv := strVal(p.ParamValue)
		pn := strVal(p.ParamName)
		if pv != "" && pn != "" {
			m[pv] = pn
		}
	}
	return m
}

func connStatus(v string) int {
	if v == "1" {
		return 1
	}
	return 0
}

func paramIds(params []device.ElementBasicInfoParameter) []int64 {
	ids := make([]int64, 0, len(params))
	for _, p := range params {
		ids = append(ids, p.ParamId)
	}
	return ids
}

// extractPort extracts the port index from a route string.
func extractPort(route string) string {
	parts := strings.Split(route, ".")
	ans := ""
	for _, s := range parts {
		if s == "0" && ans != "" {
			break
		}
		ans = s
	}
	return ans
}

// nextDevReg builds a regex for matching next-level device routes.
func nextDevReg(route string) string {
	parts := strings.Split(route, ".")
	var sb strings.Builder
	detected := false
	for i, p := range parts {
		if p != "0" || detected {
			sb.WriteString(regexp.QuoteMeta(p))
		} else {
			sb.WriteString("[1-9]+")
			detected = true
		}
		if i < len(parts)-1 {
			sb.WriteString("\\.")
		}
	}
	return sb.String()
}
