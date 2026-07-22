package mr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"nmsappsrv/internal/config"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// Service implements Java's MRManagementService + MRFileProcessor for the
// nmsappsrv rewrite. The upload endpoint (/acs-file-server/mr, served by
// filebase) calls IngestMR; parsing, CSV reporting and heatmap querying all
// happen in-process (Java splits these across the api and corefunction
// modules — here they are one service).
type Service struct {
	cfg  config.FileServerConfig
	repo *Repository
	db   *gorm.DB
}

// parseSem bounds concurrent MR parses (Java's PmConsumer uses a
// RateLimiter(100); a capacity-100 semaphore gives the same ceiling).
var parseSem = make(chan struct{}, 100)

func NewService(db *gorm.DB, cfg config.FileServerConfig) *Service {
	return &Service{cfg: cfg, repo: NewRepository(db), db: db}
}

// IngestMR records the upload and triggers asynchronous parsing. elementId is
// usually 0 for the direct device-upload path (the scheduled MRUploadTask path
// supplies it); callers may pass it via the upload query parameter.
func (s *Service) IngestMR(filePath string, elementID int64) error {
	log := MRFileLog{
		ElementID:  elementID,
		FileName:   filepath.Base(filePath),
		UploadTime: time.Now(),
	}
	if err := s.repo.CreateLog(&log); err != nil {
		logger.Errorf("mr: failed to record upload log: %v", err)
	}
	utils.SafeGo("mr-parse", func() {
		parseSem <- struct{}{}
		defer func() { <-parseSem }()
		if err := s.parseFile(filePath, elementID); err != nil {
			logger.Errorf("mr: parse failed for %s: %v", filePath, err)
		}
	})
	return nil
}

// parseFile parses an MRO/MRE file into mr_data. Mirrors Java's
// MRFileProcessor.parseMR: only files whose name contains MRO/MRE are parsed,
// and a tar.gz is decompressed (first xml entry) before parsing.
func (s *Service) parseFile(filePath string, elementID int64) error {
	base := filepath.Base(filePath)
	if !strings.Contains(base, "MRO") && !strings.Contains(base, "MRE") {
		return nil
	}
	content, err := readMRContent(filePath)
	if err != nil {
		return err
	}
	rows, err := ParseMRO(bytes.NewReader(content))
	if err != nil {
		return err
	}
	for i := range rows {
		rows[i].ElementID = elementID
	}
	if err := s.repo.SaveBatch(rows); err != nil {
		return err
	}
	logger.Infof("mr: parsed %d rows from %s", len(rows), base)
	return nil
}

// readMRContent returns the XML payload of an MRO/MRE file, transparently
// handling plain XML, gzip-of-XML and tar.gz-of-XML.
func readMRContent(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	magic := make([]byte, 2)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		// Try tar first; fall back to gzip-wrapped xml.
		if tr, terr := tar.NewReader(gz), error(nil); terr == nil {
			for {
				hdr, e := tr.Next()
				if e == io.EOF {
					break
				}
				if e != nil {
					break
				}
				if strings.HasSuffix(strings.ToLower(hdr.Name), ".xml") {
					return io.ReadAll(tr)
				}
			}
		}
		// rewind gz and read as plain gzip-of-xml
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		gz2, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz2.Close()
		return io.ReadAll(gz2)
	}
	return io.ReadAll(f)
}

// ---- CSV report (Java MRCSVReportUtil.exportMRReport) ----

// ExportMRVO mirrors Java's ExportMRVO.
type ExportMRVO struct {
	CellID                                               string `json:"cellId"`
	DeviceName                                           string `json:"deviceName"`
	SerialNumber                                         string `json:"serialNumber"`
	StartTime                                            string `json:"startTime"`
	EndTime                                              string `json:"endTime"`
	RSRPTotalNumberOfSamplingPoints                      int    `json:"RSRPTotalNumberOfSamplingPoints"`
	NumberOfEffectiveCoveringSamplingPointsGraterThan93  int    `json:"numberOfEffectiveCoveringSamplingPointsGraterThan93"`
	CoverageRateGreaterThan93                            string `json:"coverageRateGreaterThan93"`
	NumberOfEffectiveCoveringSamplingPointsGraterThan110 int    `json:"numberOfEffectiveCoveringSamplingPointsGraterThan110"`
	CoverageRateGreaterThan110                           string `json:"coverageRateGreaterThan110"`
	NumberOfEffectiveCoveringSamplingPointsGraterThan109 int    `json:"numberOfEffectiveCoveringSamplingPointsGraterThan109"`
	CoverageRateGreaterThan109                           string `json:"coverageRateGreaterThan109"`
	AverageRSRP                                          string `json:"averageRSRP"`
}

// GenerateCSV aggregates MR data per element/cell for [start,end] and writes an
// xlsx report (Java's ExportMRVO columns) to MRCCSExportDir. Returns the file
// name (without path), matching Java generateMRCSVForNR which returns fileName.
func (s *Service) GenerateCSV(tenantID int, start, end time.Time) (string, error) {
	var elements []struct {
		NeNeid       int64
		DeviceName   *string
		SerialNumber *string
	}
	if err := s.db.Table("cpe_element").
		Select("ne_neid, device_name, serial_number").
		Where("tenant_id = ?", tenantID).Find(&elements).Error; err != nil {
		return "", err
	}

	var vos []ExportMRVO
	for _, e := range elements {
		cellVOs, err := s.generateVOForElement(e.NeNeid, e.DeviceName, e.SerialNumber, start, end)
		if err != nil {
			logger.Errorf("mr: csv gen failed for element %d: %v", e.NeNeid, err)
			continue
		}
		vos = append(vos, cellVOs...)
	}

	fileName := "MR_Report_" + uuid.New().String() + ".xlsx"
	if s.cfg.MrCCSExportDir == "" {
		return "", fmt.Errorf("mr_ccs_export_dir not configured")
	}
	full := filepath.Join(s.cfg.MrCCSExportDir, fileName)
	if err := writeMRReportXLSX(full, vos); err != nil {
		return "", err
	}
	return fileName, nil
}

func (s *Service) generateVOForElement(elementID int64, devName, serial *string, start, end time.Time) ([]ExportMRVO, error) {
	rows, err := s.repo.ListByElementTime(elementID, start, end)
	if err != nil {
		return nil, err
	}
	cellSet := map[string]bool{}
	for _, r := range rows {
		if r.CellID != "" {
			cellSet[r.CellID] = true
		}
	}
	var vos []ExportMRVO
	for cellID := range cellSet {
		dn, sn := "", ""
		if devName != nil {
			dn = *devName
		}
		if serial != nil {
			sn = *serial
		}
		vos = append(vos, aggregateCell(cellID, dn, sn, start, end, rows))
	}
	return vos, nil
}

func aggregateCell(cellID, devName, serial string, start, end time.Time, all []MRData) ExportMRVO {
	var cellRows []MRData
	for _, r := range all {
		if r.CellID == cellID {
			cellRows = append(cellRows, r)
		}
	}
	rsrpNumber := len(cellRows)
	vo := ExportMRVO{
		CellID:                          cellID,
		DeviceName:                      devName,
		SerialNumber:                    serial,
		StartTime:                       start.Format("2006-01-02 15:04:05"),
		EndTime:                         end.Format("2006-01-02 15:04:05"),
		RSRPTotalNumberOfSamplingPoints: rsrpNumber,
	}
	var cov93, cov109, cov110, count int
	var avg *float64
	for _, r := range cellRows {
		n, err := parseInt(r.NRScSSRSRP)
		if err != nil {
			continue
		}
		if n >= 64 {
			cov93++
		}
		if n >= 58 {
			cov109++
		}
		if n >= 57 {
			cov110++
		}
		if n > 0 {
			rsrp := float64(-157 + n)
			if avg == nil {
				v := rsrp
				avg = &v
			} else {
				*avg = (*avg*float64(count) + rsrp) / float64(count+1)
			}
			count++
		}
	}
	vo.NumberOfEffectiveCoveringSamplingPointsGraterThan93 = cov93
	vo.NumberOfEffectiveCoveringSamplingPointsGraterThan109 = cov109
	vo.NumberOfEffectiveCoveringSamplingPointsGraterThan110 = cov110
	if rsrpNumber > 0 {
		vo.CoverageRateGreaterThan93 = rate4(cov93, rsrpNumber)
		vo.CoverageRateGreaterThan109 = rate4(cov109, rsrpNumber)
		vo.CoverageRateGreaterThan110 = rate4(cov110, rsrpNumber)
	}
	if avg != nil {
		vo.AverageRSRP = fmt.Sprintf("%.4f", *avg)
	}
	return vo
}

func rate4(num, den int) string {
	if den == 0 {
		return "0.0000"
	}
	return fmt.Sprintf("%.4f", 100.0*float64(num)/float64(den))
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

func writeMRReportXLSX(path string, vos []ExportMRVO) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "MR_Report"
	f.NewSheet(sheet)
	headers := []string{
		"Cell ID", "Device Name", "Serial Number", "Start Time", "End Time",
		"RSRP MR Num", "Valid Coverage MR Num(>=-93)", "Coverage Rate(>=-93)",
		"Valid Coverage MR Num(>=-110)", "Coverage Rate(>=-110)",
		"Valid Coverage MR Num(>=-109)", "Coverage Rate(>=-109)", "Average RSRP",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for ri, vo := range vos {
		row := ri + 2
		vals := []interface{}{
			vo.CellID, vo.DeviceName, vo.SerialNumber, vo.StartTime, vo.EndTime,
			vo.RSRPTotalNumberOfSamplingPoints,
			vo.NumberOfEffectiveCoveringSamplingPointsGraterThan93, vo.CoverageRateGreaterThan93,
			vo.NumberOfEffectiveCoveringSamplingPointsGraterThan110, vo.CoverageRateGreaterThan110,
			vo.NumberOfEffectiveCoveringSamplingPointsGraterThan109, vo.CoverageRateGreaterThan109,
			vo.AverageRSRP,
		}
		for ci, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(ci+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
	f.DeleteSheet("Sheet1")
	return f.SaveAs(path)
}

// ---- heatmap (Java NRHeatmapOnCartesianParseProcessor) ----

// HeatmapVO mirrors Java's GetMRStatisticDataForDataVO.
type HeatmapVO struct {
	StatisticType string   `json:"statisticType"`
	XAxis         []string `json:"xAxis"`
	YAxis         []string `json:"yAxis"`
	Data          [][3]int `json:"data"`
}

// HeatmapQuery mirrors Java's GetMRStatisticDataForDataQuery.
type HeatmapQuery struct {
	MRName    string    `json:"mrName"`
	ElementID int64     `json:"elementId"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

// GetHeatmap returns the heatmap buckets for the query. Bucketing thresholds
// are the raw encoded values ([<=47, <=67, >67]) exactly like Java; xAxis
// labels differ per mrName.
func (s *Service) GetHeatmap(q HeatmapQuery) (*HeatmapVO, error) {
	if q.MRName == "" || q.ElementID == 0 || q.StartTime.IsZero() || q.EndTime.IsZero() {
		return nil, fmt.Errorf("mrName, elementId, startTime and endTime are required")
	}
	xAxis := heatmapXAxis(q.MRName)
	all, err := s.repo.ListByElementTime(q.ElementID, q.StartTime, q.EndTime)
	if err != nil {
		return nil, err
	}
	vo := &HeatmapVO{XAxis: xAxis}
	if len(all) == 0 {
		vo.Data = [][3]int{}
		return vo, nil
	}
	cellSet := map[string]bool{}
	for _, r := range all {
		if r.CellID != "" {
			cellSet[r.CellID] = true
		}
	}
	for _, c := range orderedKeys(cellSet) {
		vo.YAxis = append(vo.YAxis, c)
	}
	dataTemp := make([][]int, len(xAxis))
	for i := range dataTemp {
		dataTemp[i] = make([]int, len(vo.YAxis))
	}
	for yi, cellID := range vo.YAxis {
		var cellRows []MRData
		for _, r := range all {
			if r.CellID == cellID {
				cellRows = append(cellRows, r)
			}
		}
		for _, r := range cellRows {
			v := mrFieldValue(r, q.MRName)
			n, err := parseInt(v)
			if err != nil {
				continue
			}
			var bucket int
			switch {
			case n <= 47:
				bucket = 0
			case n <= 67:
				bucket = 1
			default:
				bucket = 2
			}
			dataTemp[bucket][yi]++
		}
	}
	for i := 0; i < len(xAxis); i++ {
		for j := 0; j < len(vo.YAxis); j++ {
			if dataTemp[i][j] > 0 {
				vo.Data = append(vo.Data, [3]int{i, j, dataTemp[i][j]})
			}
		}
	}
	return vo, nil
}

func heatmapXAxis(mrName string) []string {
	switch mrName {
	case "NRScSSRSRQ", "NRNcSSRSRQ", "LteNcRSRQ":
		return []string{"Weak(<=-20)", "Normal(>-20 & <=-10)", "Strong(>-10)"}
	case "NRScSSRSRP", "NRNcSSRSRP", "LteNcRSRP":
		return []string{"Weak(<=-110)", "Normal(>-110 & <=-90)", "Strong(>-90)"}
	default:
		return []string{"Weak", "Normal", "Strong"}
	}
}

func mrFieldValue(r MRData, mrName string) string {
	switch mrName {
	case "NRNcSSRSRQ":
		return r.NRNcSSRSRQ
	case "NRNcSSRSRP":
		return r.NRNcSSRSRP
	case "NRScSSRSRQ":
		return r.NRScSSRSRQ
	case "NRScSSRSRP":
		return r.NRScSSRSRP
	case "LteNcRSRP":
		return r.LteNcRSRP
	case "LteNcRSRQ":
		return r.LteNcRSRQ
	default:
		return ""
	}
}

func orderedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---- logs / raw file ----

func (s *Service) ListLogs(page, pageSize int) ([]MRFileLog, int64, error) {
	return s.repo.ListLogs(page, pageSize)
}

// RawFilePath returns the on-disk path of an uploaded MR file (for download).
func (s *Service) RawFilePath(id int64) (string, error) {
	log, err := s.repo.GetLogByID(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.cfg.MrDir, log.FileName), nil
}

// ReportFilePath returns the on-disk path of a generated CSV report.
func (s *Service) ReportFilePath(fileName string) (string, error) {
	if !safeReportName(fileName) {
		return "", fmt.Errorf("invalid report file name")
	}
	return filepath.Join(s.cfg.MrCCSExportDir, fileName), nil
}

func safeReportName(name string) bool {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return false
	}
	return true
}
