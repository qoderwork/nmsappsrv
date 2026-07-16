package corenet

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

//go:embed templates/*.xlsx
var pcfTemplateFS embed.FS

// ImportPCFUEDTO mirrors Java ImportPCFUEDTO (the PCF UE template columns).
type ImportPCFUEDTO struct {
	Imsi      string `json:"imsi"`
	Msisdn    string `json:"msisdn"`
	Rfsp      *int   `json:"rfsp"`
	Sar       string `json:"sar"`
	PccRules  string `json:"pccRules"`
	SessRules string `json:"sessRules"`
	UePolicy  string `json:"uePolicy"`
	QosAudio  string `json:"qosAudio"`
	QosVideo  string `json:"qosVideo"`
	HdrEnrich string `json:"hdrEnrich"`
}

// PCFUEVO mirrors Java PCFUEVO (extends ImportPCFUEDTO + coreNetworkId).
type PCFUEVO struct {
	ImportPCFUEDTO
	CoreNetworkId int `json:"coreNetworkId"`
}

// DeletePCFUEDTO mirrors Java DeletePCFUEDTO.
type DeletePCFUEDTO struct {
	Imsi          string `json:"imsi"`
	Msisdn        string `json:"msisdn"`
	CoreNetworkId int    `json:"coreNetworkId"`
}

// pcfUEColumns is the fixed template header order, mirroring the Java
// ImportPCFUEDTO field order used by EasyExcel.
var pcfUEColumns = []string{"imsi", "msisdn", "rfsp", "sar", "pccRules", "sessRules", "uePolicy", "qosAudio", "qosVideo", "hdrEnrich"}

// parsePCFUEExcel reads the "UE Info" sheet and maps rows to ImportPCFUEDTO,
// mirroring Java EasyExcel.read(...).sheet("UE Info"). The first row is the
// header; empty rows are skipped.
func parsePCFUEExcel(r io.Reader) ([]ImportPCFUEDTO, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := f.GetRows("UE Info")
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("PCF UE sheet is empty")
	}
	idx := make(map[string]int, len(pcfUEColumns))
	for i, h := range rows[0] {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	out := make([]ImportPCFUEDTO, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if rowEmpty(row) {
			continue
		}
		d := ImportPCFUEDTO{}
		if i, ok := idx["imsi"]; ok && i < len(row) {
			d.Imsi = row[i]
		}
		if i, ok := idx["msisdn"]; ok && i < len(row) {
			d.Msisdn = row[i]
		}
		if i, ok := idx["rfsp"]; ok && i < len(row) && row[i] != "" {
			if v, e := strconv.Atoi(row[i]); e == nil {
				d.Rfsp = &v
			}
		}
		if i, ok := idx["sar"]; ok && i < len(row) {
			d.Sar = row[i]
		}
		if i, ok := idx["pccrules"]; ok && i < len(row) {
			d.PccRules = row[i]
		}
		if i, ok := idx["sessrules"]; ok && i < len(row) {
			d.SessRules = row[i]
		}
		if i, ok := idx["uepolicy"]; ok && i < len(row) {
			d.UePolicy = row[i]
		}
		if i, ok := idx["qosaudio"]; ok && i < len(row) {
			d.QosAudio = row[i]
		}
		if i, ok := idx["qosvideo"]; ok && i < len(row) {
			d.QosVideo = row[i]
		}
		if i, ok := idx["hdrenrich"]; ok && i < len(row) {
			d.HdrEnrich = row[i]
		}
		out = append(out, d)
	}
	return out, nil
}

func rowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// DownloadPCFUETemplate handles POST /core-networks/pcf-ue/template
// Serves the PCF UE import template xlsx (mirrors Java downloadUPFUETemplate,
// which serves the classpath "PCF UE Template.xlsx").
func (h *Handler) DownloadPCFUETemplate(c *gin.Context) {
	data, err := h.svc.DownloadPCFUETemplate()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="PCF UE Template.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ImportPCFUE handles POST /core-networks/pcf-ue/import
// Multipart file (form field "file") + coreNetworkId. Parses the xlsx,
// validates, forwards each UE to the PCF element's 33030 ueManagement API,
// and logs the operation. Mirrors Java importPCFUE.
func (h *Handler) ImportPCFUE(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "file is required")
		return
	}
	coreNetworkId, _ := strconv.Atoi(c.PostForm("coreNetworkId"))
	f, err := fileHeader.Open()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	defer f.Close()
	if err := h.svc.ImportPCFUE(f, coreNetworkId, middleware.GetUsername(c)); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// UpdatePCFUE handles POST /core-networks/pcf-ue/update
// Body: PCFUEVO. PUTs the UE to the PCF element's 33030 ueManagement API and
// logs the operation. Mirrors Java updatePCFUE.
func (h *Handler) UpdatePCFUE(c *gin.Context) {
	var dto PCFUEVO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.UpdatePCFUE(dto, middleware.GetUsername(c)); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}

// DeletePCFUE handles POST /core-networks/pcf-ue/delete
// Body: DeletePCFUEDTO. DELETEs the UE from the PCF element's 33030
// ueManagement API and logs the operation. Mirrors Java deletePCFUE.
func (h *Handler) DeletePCFUE(c *gin.Context) {
	var dto DeletePCFUEDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.DeletePCFUE(dto, middleware.GetUsername(c)); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}
