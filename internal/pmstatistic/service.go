package pmstatistic

import (
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	"nmsappsrv/pkg/logger"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type ElementInfo struct {
	NeNeid      int64   `gorm:"column:ne_neid"`
	ModelName   *string `gorm:"column:model_name"`
	SerialNumber *string `gorm:"serial_number"`
}

type PMFileInfo struct {
	FileName string `gorm:"column:file_name"`
	NeId     int64  `gorm:"column:ne_id"`
}

type ParameterValue struct {
	ElementId  int64  `gorm:"column:element_id"`
	ParamName  string `gorm:"column:param_name"`
	ParamValue string `gorm:"column:param_value"`
}

func (s *Service) ExportPM() {
	start := time.Now()
	defer func() {
		logger.Infof("PMDataStatisticTask: completed in %v", time.Since(start))
	}()

	now := time.Now()
	endTime := now.Truncate(time.Hour)
	startTime := endTime.Add(-1 * time.Hour)
	dateStr := startTime.Format("20060102")

	var elements []ElementInfo
	if err := s.db.Table("cpe_element").
		Select("ne_neid, model_name, serial_number").
		Where("deleted = 0").
		Scan(&elements).Error; err != nil {
		logger.Errorf("PMDataStatisticTask: failed to get elements: %v", err)
		return
	}

	idToElement := make(map[int64]*ElementInfo)
	for i := range elements {
		idToElement[elements[i].NeNeid] = &elements[i]
	}

	var params []ParameterValue
	if err := s.db.Table("element_basic_info_parameter").
		Select("element_id, param_name, param_value").
		Where("param_name IN ?", []string{
			"Device.Services.FAPService.1.FAPControl.NR.RAN.Common.gNBId",
			"Device.Services.FAPService.1.FAPControl.NR.RAN.Common.gNBName",
		}).
		Scan(&params).Error; err != nil {
		logger.Errorf("PMDataStatisticTask: failed to get parameters: %v", err)
		return
	}

	idToGnbId := make(map[int64]string)
	idToGnbName := make(map[int64]string)
	for _, p := range params {
		switch p.ParamName {
		case "Device.Services.FAPService.1.FAPControl.NR.RAN.Common.gNBId":
			idToGnbId[p.ElementId] = p.ParamValue
		case "Device.Services.FAPService.1.FAPControl.NR.RAN.Common.gNBName":
			idToGnbName[p.ElementId] = p.ParamValue
		}
	}

	var tenancies []struct {
		Id int `gorm:"column:id"`
	}
	if err := s.db.Table("tenancy").Select("id").Scan(&tenancies).Error; err != nil {
		logger.Errorf("PMDataStatisticTask: failed to get tenancies: %v", err)
		return
	}

	for _, tenancy := range tenancies {
		var fileInfos []PMFileInfo
		tableNames := make([]string, 100)
		for i := 0; i < 100; i++ {
			tableNames[i] = fmt.Sprintf("pm_file_log_%d", i)
		}

		for _, tableName := range tableNames {
			var temp []PMFileInfo
			err := s.db.Table(tableName).
				Select("file_name, ne_id").
				Where("tenancy_id = ? AND start_time >= ? AND start_time < ?",
					tenancy.Id, startTime, endTime).
				Scan(&temp).Error
			if err != nil {
				continue
			}
			fileInfos = append(fileInfos, temp...)
		}

		if len(fileInfos) == 0 {
			continue
		}

		elementFileMap := make(map[int64][]string)
		for _, fi := range fileInfos {
			elementFileMap[fi.NeId] = append(elementFileMap[fi.NeId], fi.FileName)
		}

		gnbResourceUtilizations := make([]GNBResourceUtilization, 0)
		nrCellCURrcConnections := make([]NRCellCURrcConnection, 0)
		nrCellCUQosFlows := make([]NRCellCUQosFlow, 0)
		nrCellCURrcConnectedUserRDSWps := make([]NRCellCURrcConnectedUserRD, 0)
		nrCellCURrcConnectedUserRDSNonWps := make([]NRCellCURrcConnectedUserRD, 0)
		nrCellDUPagingRlcs := make([]NRCellDUPagingRlc, 0)
		nrCellDURlcsWps := make([]NRCellDURlc, 0)
		nrCellDURlcsNonWps := make([]NRCellDURlc, 0)
		nrCellDUMacPrbs := make([]NRCellDUMacPrb, 0)
		nrCellDUMacsWps := make([]NRCellDUMac, 0)
		nrCellDUMacsNonWps := make([]NRCellDUMac, 0)
		nrCellCUUPPdcps := make([]NRCellCUUPPdcp, 0)
		nrCellCUUPPdcpsWps := make([]NRCellCUUPPdcp, 0)
		nrCellCUUPPdcpsNonWps := make([]NRCellCUUPPdcp, 0)
		mobilitiesWps := make([]Mobility, 0)
		mobilitiesNonWps := make([]Mobility, 0)

		cellAvailabilities := make([]PMCellAvailability, 0)
		volumeAvailabilities := make([]PMDataVolumeAvailability, 0)
		qoSFlows := make([]PMQoSFlow, 0)
		pmrrcs := make([]PMRRC, 0)
		pmueContexts := make([]PMUEContext, 0)
		pmMobilities := make([]PMMobility, 0)
		pmThroughputs := make([]PMThroughput, 0)
		utilizations := make([]Utilization, 0)
		ngSignalings := make([]NgSignaling, 0)

		for elementId := range elementFileMap {
			element, ok := idToElement[elementId]
			if !ok {
				continue
			}

			var measurements []struct {
				KPIName       string    `gorm:"column:kpi_name"`
				KPIValue      string    `gorm:"column:measured_value"`
				MeasObjLdn    string    `gorm:"column:meas_obj_ldn"`
				CellIdentity  string    `gorm:"column:cell_identity"`
				MeasureTime   time.Time `gorm:"column:measure_time"`
			}

			err := s.db.Table("pm_kpi_measurement").
				Select("kpi_name, measured_value, meas_obj_ldn, cell_identity, measure_time").
				Where("element_id = ? AND measure_time >= ? AND measure_time < ?",
					elementId, startTime, endTime).
				Scan(&measurements).Error
			if err != nil {
				logger.Warnf("PMDataStatisticTask: failed to get measurements for element %d: %v", elementId, err)
				continue
			}

			timeMap := make(map[time.Time][]*struct {
				KPIName    string
				KPIValue   string
				MeasObjLdn string
			})

			for i := range measurements {
				m := &measurements[i]
				timeMap[m.MeasureTime] = append(timeMap[m.MeasureTime], &struct {
					KPIName    string
					KPIValue   string
					MeasObjLdn string
				}{
					KPIName:    m.KPIName,
					KPIValue:   m.KPIValue,
					MeasObjLdn: m.MeasObjLdn,
				})
			}

			for t, dataInTime := range timeMap {
				s.processGNBResourceUtilization(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &gnbResourceUtilizations)
				s.processNRCellCURrcConnection(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCURrcConnections)
				s.processNRCellCUQosFlow(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCUQosFlows)
				s.processNRCellCURrcConnectedUserRD(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCURrcConnectedUserRDSWps, true)
				s.processNRCellCURrcConnectedUserRD(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCURrcConnectedUserRDSNonWps, false)
				s.processNRCellDUPagingRlc(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellDUPagingRlcs)
				s.processNRCellDURlc(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellDURlcsWps, true)
				s.processNRCellDURlc(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellDURlcsNonWps, false)
				s.processNRCellDUMacPrb(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellDUMacPrbs)
				s.processNRCellDUMac(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellDUMacsWps, true)
				s.processNRCellDUMac(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellDUMacsNonWps, false)
				s.processNRCellCUUPPdcp(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCUUPPdcps, false)
				s.processNRCellCUUPPdcp(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCUUPPdcpsWps, true)
				s.processNRCellCUUPPdcp(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &nrCellCUUPPdcpsNonWps, false)
				s.processMobility(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &mobilitiesWps, true)
				s.processMobility(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &mobilitiesNonWps, false)

				s.processCellAvailability(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &cellAvailabilities)
				s.processVolume(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &volumeAvailabilities)
				s.processQoSFlow(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &qoSFlows)
				s.processRRC(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &pmrrcs)
				s.processUEContext(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &pmueContexts)
				s.processPMMobility(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &pmMobilities)
				s.processThroughput(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &pmThroughputs)
				s.processUtilization(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &utilizations)
				s.processNgSignaling(elementId, t, dataInTime, element, idToGnbId, idToGnbName, &ngSignalings)
			}
		}

		basePath := "/data/northbound"
		localPath := filepath.Join(basePath, fmt.Sprintf("%d", tenancy.Id), "PM", dateStr)

		s.exportAllCSV(localPath, startTime, endTime,
			gnbResourceUtilizations,
			nrCellCURrcConnections,
			nrCellCUQosFlows,
			nrCellCURrcConnectedUserRDSWps,
			nrCellCURrcConnectedUserRDSNonWps,
			nrCellDUPagingRlcs,
			nrCellDURlcsWps,
			nrCellDURlcsNonWps,
			nrCellDUMacPrbs,
			nrCellDUMacsWps,
			nrCellDUMacsNonWps,
			nrCellCUUPPdcps,
			nrCellCUUPPdcpsWps,
			nrCellCUUPPdcpsNonWps,
			mobilitiesWps,
			mobilitiesNonWps,
			cellAvailabilities,
			volumeAvailabilities,
			qoSFlows,
			pmrrcs,
			pmueContexts,
			pmMobilities,
			pmThroughputs,
			utilizations,
			ngSignalings,
		)
	}
}

func (s *Service) processGNBResourceUtilization(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]GNBResourceUtilization) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "cpuUsage" || d.KPIName == "memUsage" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := GNBResourceUtilization{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "cpuUsage=") {
				item.CpuUsage = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "memUsage=") {
				item.MemUsage = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellCURrcConnection(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellCURrcConnection) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "rrcConnAvg" || d.KPIName == "rrcConnMax" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellCURrcConnection{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "rrcConnAvg=") {
				item.RrcConnAvg = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "rrcConnMax=") {
				item.RrcConnMax = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellCUQosFlow(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellCUQosFlow) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "qosFlowAvg" || d.KPIName == "qosFlowMax" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellCUQosFlow{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "qosFlowAvg=") {
				item.QosFlowAvg = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "qosFlowMax=") {
				item.QosFlowMax = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellCURrcConnectedUserRD(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellCURrcConnectedUserRD, isWPS bool) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "rrcConnUsers" {
			wpsMatch := IsWPS(d.MeasObjLdn)
			if wpsMatch == isWPS {
				ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
			}
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellCURrcConnectedUserRD{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "rrcConnUsers=") {
				item.RrcConnUsers = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellDUPagingRlc(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellDUPagingRlc) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "pagingRlc" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellDUPagingRlc{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "pagingRlc=") {
				item.PagingRlc = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellDURlc(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellDURlc, isWPS bool) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "rlcUlBytes" || d.KPIName == "rlcDlBytes" {
			wpsMatch := IsWPS(d.MeasObjLdn)
			if wpsMatch == isWPS {
				ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
			}
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellDURlc{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "rlcUlBytes=") {
				item.RlcUlBytes = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "rlcDlBytes=") {
				item.RlcDlBytes = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellDUMacPrb(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellDUMacPrb) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "prbUtilization" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellDUMacPrb{
			Plmn:            ldnDto.Plmn,
			LocalNrCI:       ldnDto.LocalNrCI,
			DeviceName:      strVal(element.ModelName),
			SerialNumber:    strVal(element.SerialNumber),
			GnbId:           idToGnbId[elementId],
			GnbName:         idToGnbName[elementId],
			BeginTime:       formatTime(t),
			EndTime:         formatTime(t.Add(15 * time.Minute)),
			PrbUtilization: parseFloat(stringsSplit(ldnMap[ldn][0], "=")[1]),
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellDUMac(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellDUMac, isWPS bool) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "macUlBytes" || d.KPIName == "macDlBytes" {
			wpsMatch := IsWPS(d.MeasObjLdn)
			if wpsMatch == isWPS {
				ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
			}
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellDUMac{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "macUlBytes=") {
				item.MacUlBytes = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "macDlBytes=") {
				item.MacDlBytes = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNRCellCUUPPdcp(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NRCellCUUPPdcp, isWPS bool) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "pdcpUlBytes" || d.KPIName == "pdcpDlBytes" {
			wpsMatch := IsWPS(d.MeasObjLdn)
			if wpsMatch == isWPS {
				ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
			}
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NRCellCUUPPdcp{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "pdcpUlBytes=") {
				item.PdcpUlBytes = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "pdcpDlBytes=") {
				item.PdcpDlBytes = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processMobility(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]Mobility, isWPS bool) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "handoverSucc" || d.KPIName == "handoverAtt" {
			wpsMatch := IsWPS(d.MeasObjLdn)
			if wpsMatch == isWPS {
				ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
			}
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := Mobility{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			Remote:       ldnDto.Remote,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "handoverSucc=") {
				item.HandoverSucc = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "handoverAtt=") {
				item.HandoverAtt = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processCellAvailability(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMCellAvailability) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "cellAvailability" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMCellAvailability{
			Plmn:             ldnDto.Plmn,
			LocalNrCI:        ldnDto.LocalNrCI,
			DeviceName:       strVal(element.ModelName),
			SerialNumber:     strVal(element.SerialNumber),
			GnbId:            idToGnbId[elementId],
			GnbName:          idToGnbName[elementId],
			BeginTime:        formatTime(t),
			EndTime:          formatTime(t.Add(15 * time.Minute)),
			CellAvailability: parseFloat(stringsSplit(ldnMap[ldn][0], "=")[1]),
		}
		*result = append(*result, item)
	}
}

func (s *Service) processVolume(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMDataVolumeAvailability) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "ulVolume" || d.KPIName == "dlVolume" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMDataVolumeAvailability{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "ulVolume=") {
				item.UlVolume = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "dlVolume=") {
				item.DlVolume = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processQoSFlow(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMQoSFlow) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "qosFlowCount" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMQoSFlow{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
			QosFlowCount: parseFloat(stringsSplit(ldnMap[ldn][0], "=")[1]),
		}
		*result = append(*result, item)
	}
}

func (s *Service) processRRC(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMRRC) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "rrcConnCount" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMRRC{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
			RrcConnCount: parseFloat(stringsSplit(ldnMap[ldn][0], "=")[1]),
		}
		*result = append(*result, item)
	}
}

func (s *Service) processUEContext(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMUEContext) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "ueContextAvg" || d.KPIName == "ueContextMax" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMUEContext{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "ueContextAvg=") {
				item.UeContextAvg = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "ueContextMax=") {
				item.UeContextMax = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processPMMobility(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMMobility) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "intraHoCount" || d.KPIName == "interHoCount" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMMobility{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "intraHoCount=") {
				item.IntraHoCount = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "interHoCount=") {
				item.InterHoCount = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processThroughput(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]PMThroughput) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "ulThroughput" || d.KPIName == "dlThroughput" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := PMThroughput{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "ulThroughput=") {
				item.UlThroughput = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "dlThroughput=") {
				item.DlThroughput = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processUtilization(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]Utilization) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "cpuUtil" || d.KPIName == "memUtil" || d.KPIName == "rrcUtil" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := Utilization{
			Plmn:         ldnDto.Plmn,
			LocalNrCI:    ldnDto.LocalNrCI,
			DeviceName:   strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:        idToGnbId[elementId],
			GnbName:      idToGnbName[elementId],
			BeginTime:    formatTime(t),
			EndTime:      formatTime(t.Add(15 * time.Minute)),
		}

		for _, kv := range ldnMap[ldn] {
			if stringsContains(kv, "cpuUtil=") {
				item.CpuUtil = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "memUtil=") {
				item.MemUtil = parseFloat(stringsSplit(kv, "=")[1])
			} else if stringsContains(kv, "rrcUtil=") {
				item.RrcUtil = parseFloat(stringsSplit(kv, "=")[1])
			}
		}
		*result = append(*result, item)
	}
}

func (s *Service) processNgSignaling(elementId int64, t time.Time, dataInTime []*struct {
	KPIName    string
	KPIValue   string
	MeasObjLdn string
}, element *ElementInfo, idToGnbId, idToGnbName map[int64]string, result *[]NgSignaling) {
	ldnMap := make(map[string][]string)
	for _, d := range dataInTime {
		if d.KPIName == "ngSigCount" {
			ldnMap[d.MeasObjLdn] = append(ldnMap[d.MeasObjLdn], d.KPIName+"="+d.KPIValue)
		}
	}

	for ldn := range ldnMap {
		ldnDto := ParseLDN(ldn)
		item := NgSignaling{
			Plmn:       ldnDto.Plmn,
			LocalNrCI:  ldnDto.LocalNrCI,
			DeviceName: strVal(element.ModelName),
			SerialNumber: strVal(element.SerialNumber),
			GnbId:      idToGnbId[elementId],
			GnbName:    idToGnbName[elementId],
			BeginTime:  formatTime(t),
			EndTime:    formatTime(t.Add(15 * time.Minute)),
			NgSigCount: parseFloat(stringsSplit(ldnMap[ldn][0], "=")[1]),
		}
		*result = append(*result, item)
	}
}

func (s *Service) exportAllCSV(localPath string, startTime, endTime time.Time,
	gnbResourceUtilizations []GNBResourceUtilization,
	nrCellCURrcConnections []NRCellCURrcConnection,
	nrCellCUQosFlows []NRCellCUQosFlow,
	nrCellCURrcConnectedUserRDSWps []NRCellCURrcConnectedUserRD,
	nrCellCURrcConnectedUserRDSNonWps []NRCellCURrcConnectedUserRD,
	nrCellDUPagingRlcs []NRCellDUPagingRlc,
	nrCellDURlcsWps []NRCellDURlc,
	nrCellDURlcsNonWps []NRCellDURlc,
	nrCellDUMacPrbs []NRCellDUMacPrb,
	nrCellDUMacsWps []NRCellDUMac,
	nrCellDUMacsNonWps []NRCellDUMac,
	nrCellCUUPPdcps []NRCellCUUPPdcp,
	nrCellCUUPPdcpsWps []NRCellCUUPPdcp,
	nrCellCUUPPdcpsNonWps []NRCellCUUPPdcp,
	mobilitiesWps []Mobility,
	mobilitiesNonWps []Mobility,
	cellAvailabilities []PMCellAvailability,
	volumeAvailabilities []PMDataVolumeAvailability,
	qoSFlows []PMQoSFlow,
	pmrrcs []PMRRC,
	pmueContexts []PMUEContext,
	pmMobilities []PMMobility,
	pmThroughputs []PMThroughput,
	utilizations []Utilization,
	ngSignalings []NgSignaling,
) {
	timeFormat := "20060102150405"
	stStr := startTime.Format(timeFormat)
	etStr := endTime.Format(timeFormat)

	csvTasks := []struct {
		data     interface{}
		fileName string
	}{
		{gnbResourceUtilizations, "GNB_Resource_Utilization_" + stStr + "_" + etStr + ".csv"},
		{nrCellCURrcConnections, "NRCellCU_RrcConnection_" + stStr + "_" + etStr + ".csv"},
		{nrCellCUQosFlows, "NRCellCU_QosFlow_" + stStr + "_" + etStr + ".csv"},
		{nrCellCURrcConnectedUserRDSWps, "NRCellCU_RrcConnectedUserRD_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellCURrcConnectedUserRDSNonWps, "NRCellCU_RrcConnectedUserRD_Non_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellDUPagingRlcs, "NRCellDU_PagingRlc_" + stStr + "_" + etStr + ".csv"},
		{nrCellDURlcsWps, "NRCellDU_Rlc_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellDURlcsNonWps, "NRCellDU_Rlc_Non_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellDUMacPrbs, "NRCellDU_MacPrb_" + stStr + "_" + etStr + ".csv"},
		{nrCellDUMacsWps, "NRCellDU_Mac_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellDUMacsNonWps, "NRCellDU_Mac_Non_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellCUUPPdcps, "NRCellCUUP_Pdcp_" + stStr + "_" + etStr + ".csv"},
		{nrCellCUUPPdcpsWps, "NRCellCUUP_Pdcp_WPS_" + stStr + "_" + etStr + ".csv"},
		{nrCellCUUPPdcpsNonWps, "NRCellCUUP_Pdcp_Non_WPS_" + stStr + "_" + etStr + ".csv"},
		{mobilitiesWps, "Mobility_WPS_" + stStr + "_" + etStr + ".csv"},
		{mobilitiesNonWps, "Mobility_Non_WPS_" + stStr + "_" + etStr + ".csv"},
		{cellAvailabilities, "PM_CellAvailability_" + stStr + "_" + etStr + ".csv"},
		{volumeAvailabilities, "PM_Volume_" + stStr + "_" + etStr + ".csv"},
		{qoSFlows, "PM_QoSFlow_" + stStr + "_" + etStr + ".csv"},
		{pmrrcs, "PM_RRCConnection_" + stStr + "_" + etStr + ".csv"},
		{pmueContexts, "PM_UEContexts_" + stStr + "_" + etStr + ".csv"},
		{pmMobilities, "PM_Mobility_" + stStr + "_" + etStr + ".csv"},
		{pmThroughputs, "PM_Throughput_" + stStr + "_" + etStr + ".csv"},
		{utilizations, "PM_Utilization_" + stStr + "_" + etStr + ".csv"},
		{ngSignalings, "PM_NgSignaling_" + stStr + "_" + etStr + ".csv"},
	}

	for _, task := range csvTasks {
		switch v := task.data.(type) {
		case []GNBResourceUtilization:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellCURrcConnection:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellCUQosFlow:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellCURrcConnectedUserRD:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellDUPagingRlc:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellDURlc:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellDUMacPrb:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellDUMac:
			go exportGoroutine(v, localPath, task.fileName)
		case []NRCellCUUPPdcp:
			go exportGoroutine(v, localPath, task.fileName)
		case []Mobility:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMCellAvailability:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMDataVolumeAvailability:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMQoSFlow:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMRRC:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMUEContext:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMMobility:
			go exportGoroutine(v, localPath, task.fileName)
		case []PMThroughput:
			go exportGoroutine(v, localPath, task.fileName)
		case []Utilization:
			go exportGoroutine(v, localPath, task.fileName)
		case []NgSignaling:
			go exportGoroutine(v, localPath, task.fileName)
		}
	}
}

func exportGoroutine[T any](data []T, dirPath, fileName string) {
	if err := ExportCSV(data, dirPath, fileName); err != nil {
		logger.Errorf("PMDataStatisticTask: failed to export CSV %s: %v", fileName, err)
	} else {
		logger.Infof("PMDataStatisticTask: exported CSV %s (%d records)", fileName, len(data))
	}
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
