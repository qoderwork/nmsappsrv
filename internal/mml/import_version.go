package mml

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/utils"
)

// ---------------------------------------------------------------------------
// Response VOs (对齐 Java comm.vo.MmlSetVo / MmlVersionInfoVO)
// ---------------------------------------------------------------------------

// MmlSetVo is the tree node returned for the MML command catalog.
// Folders carry Name + Children; leaf commands carry IsCommand=true + CommandId.
type MmlSetVo struct {
	Name      string      `json:"name"`
	CommandId *int        `json:"commandId,omitempty"`
	IsCommand bool        `json:"isCommand"`
	Hint      *string     `json:"hint,omitempty"`
	HelpFile  *string     `json:"helpFile,omitempty"`
	Children  []MmlSetVo  `json:"children,omitempty"`
}

// MmlVersionInfoVO mirrors Java's MmlVersionInfoVO: a version label plus the
// full command tree imported under that version.
type MmlVersionInfoVO struct {
	Version   string     `json:"version"`
	ImportTime *time.Time `json:"importTime"`
	Commands  []MmlSetVo `json:"commands"`
}

// ---------------------------------------------------------------------------
// Import XML model (对齐 Java importMMLAndParameter VTDNav structure)
// ---------------------------------------------------------------------------

type mmlImportRoot struct {
	XMLName xml.Name    `xml:"root"`
	Name    string      `xml:"name,attr"`
	Mmls    []mmlFolder `xml:"mml"`
	Hints   []mmlHint   `xml:"hint"`
}

type mmlFolder struct {
	Name     string       `xml:"name,attr"`
	Type     string       `xml:"type,attr"`
	ParaObjs []mmlParaObj `xml:"paraObj"`
	Children []mmlFolder  `xml:"mml"`
}

type mmlParaObj struct {
	CmdLabel string      `xml:"cmd_label,attr"`
	Name     string      `xml:"name,attr"`
	HelpFile string      `xml:"help_file,attr"`
	Hint     string      `xml:"hint,attr"`
	CmdType  string      `xml:"cmd_type,attr"`
	Params   []mmlParam  `xml:"param"`
}

type mmlParam struct {
	Default  string `xml:"default,attr"`
	Name     string `xml:"name,attr"`
	Relate   string `xml:"relate,attr"`
	Id       string `xml:"id,attr"`
	Attr     string `xml:"attr,attr"`
	Type     string `xml:"type,attr"`
	Writable string `xml:"writable,attr"`
	Option   string `xml:"option,attr"`
	Range    string `xml:"range,attr"`
}

type mmlHint struct {
	Id   string `xml:"Id,attr"`
	Info string `xml:"Info,attr"`
}

// ---------------------------------------------------------------------------
// Local pointer helpers (avoid clashing with pkg/database.*)
// ---------------------------------------------------------------------------

func mmlStrPtr(s string) *string { return &s }
func mmlIntPtr(i int) *int       { return &i }
func mmlTimePtr(t time.Time) *time.Time { return &t }
func mmlDerefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---------------------------------------------------------------------------
// Repository additions (implemented on *repository)
// ---------------------------------------------------------------------------

// CreateMmlSet inserts a new MmlSet (folder) row.
func (r *repository) CreateMmlSet(set *MmlSet) error {
	return r.db.Create(set).Error
}

// CreateMmlCommand inserts a new MmlCommand (leaf command) row.
func (r *repository) CreateMmlCommand(cmd *MmlCommand) error {
	return r.db.Create(cmd).Error
}

// CreateMmlCommandParam inserts a new MmlCommandParam row.
func (r *repository) CreateMmlCommandParam(p *MmlCommandParam) error {
	return r.db.Create(p).Error
}

// FindMmlSetsByVersionAndLicense returns every MmlSet for the version+license
// (top-level and nested), 对齐 Java mmlSetService.getByVersionAndLicenseId.
func (r *repository) FindMmlSetsByVersionAndLicense(version string, licenseId int) ([]MmlSet, error) {
	var sets []MmlSet
	if err := r.db.Where("version = ? AND license_id = ?", version, licenseId).Find(&sets).Error; err != nil {
		logger.Errorf("FindMmlSetsByVersionAndLicense error: %v", err)
		return nil, err
	}
	return sets, nil
}

// FindTopMmlSets returns only the top-level MmlSet rows (parent_id IS NULL) for
// the version+license.
func (r *repository) FindTopMmlSets(version string, licenseId int) ([]MmlSet, error) {
	var sets []MmlSet
	if err := r.db.Where("version = ? AND license_id = ? AND parent_id IS NULL", version, licenseId).Find(&sets).Error; err != nil {
		logger.Errorf("FindTopMmlSets error: %v", err)
		return nil, err
	}
	return sets, nil
}

// FindChildMmlSets returns the direct child folders of a parent MmlSet.
func (r *repository) FindChildMmlSets(parentId int) ([]MmlSet, error) {
	var sets []MmlSet
	if err := r.db.Where("parent_id = ?", parentId).Find(&sets).Error; err != nil {
		logger.Errorf("FindChildMmlSets error: %v", err)
		return nil, err
	}
	return sets, nil
}

// FindMmlSetByParentIdAndName resolves an existing MmlSet by parent + name +
// license (used for idempotent import, 对齐 Java getByParentIdAndName).
func (r *repository) FindMmlSetByParentIdAndName(parentId *int, name string, licenseId int) ([]MmlSet, error) {
	var sets []MmlSet
	q := r.db.Where("name = ? AND license_id = ?", name, licenseId)
	if parentId != nil {
		q = q.Where("parent_id = ?", *parentId)
	} else {
		q = q.Where("parent_id IS NULL")
	}
	if err := q.Find(&sets).Error; err != nil {
		logger.Errorf("FindMmlSetByParentIdAndName error: %v", err)
		return nil, err
	}
	return sets, nil
}

// FindMmlCommandsBySetIds returns commands belonging to any of the given sets.
func (r *repository) FindMmlCommandsBySetIds(ids []int) ([]MmlCommand, error) {
	var cmds []MmlCommand
	if len(ids) == 0 {
		return cmds, nil
	}
	if err := r.db.Where("mml_set_id IN ?", ids).Find(&cmds).Error; err != nil {
		logger.Errorf("FindMmlCommandsBySetIds error: %v", err)
		return nil, err
	}
	return cmds, nil
}

// FindMmlCommandParamsByCommandIds returns params for any of the given commands.
func (r *repository) FindMmlCommandParamsByCommandIds(ids []int) ([]MmlCommandParam, error) {
	var params []MmlCommandParam
	if len(ids) == 0 {
		return params, nil
	}
	if err := r.db.Where("mml_command_id IN ?", ids).Find(&params).Error; err != nil {
		logger.Errorf("FindMmlCommandParamsByCommandIds error: %v", err)
		return nil, err
	}
	return params, nil
}

// FindMmlVersions returns the distinct, non-null versions for the license,
// 对齐 Java mmlSetService.getByLicenseId -> distinct version.
func (r *repository) FindMmlVersions(licenseId int) ([]string, error) {
	var versions []string
	if err := r.db.Model(&MmlSet{}).
		Where("license_id = ? AND version IS NOT NULL", licenseId).
		Distinct("version").
		Pluck("version", &versions).Error; err != nil {
		logger.Errorf("FindMmlVersions error: %v", err)
		return nil, err
	}
	return versions, nil
}

// DeleteMmlSetsByIds cascade-deletes MmlSet rows by id.
func (r *repository) DeleteMmlSetsByIds(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&MmlSet{}).Error
}

// DeleteMmlCommandsByIds cascade-deletes MmlCommand rows by id.
func (r *repository) DeleteMmlCommandsByIds(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&MmlCommand{}).Error
}

// DeleteMmlCommandParamsByIds cascade-deletes MmlCommandParam rows by id.
func (r *repository) DeleteMmlCommandParamsByIds(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&MmlCommandParam{}).Error
}

// ---------------------------------------------------------------------------
// Service additions (implemented on *service)
// ---------------------------------------------------------------------------

// ImportMMLAndParameter parses an MML definition XML and (re)builds the MmlSet /
// MmlCommand / MmlCommandParam tree for the given version+license. When version
// is non-empty, any existing data for that version+license is cascade-deleted
// first (对齐 Java importMMLAndParameter version-replace semantics).
//
// NOTE: Java's importMMLAndParameter also rebuilds the self-developed
// ParameterSet/Parameter catalog and ErrorInfo from the same XML. Those belong
// to the parameter/errorinfo domains and are intentionally out of scope here;
// this implementation covers the MML command tree only.
func (s *service) ImportMMLAndParameter(reader io.Reader, version string, licenseId int) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return apperror.Wrap(err, "IMPORT_MML_READ_FAILED", 500, "failed to read MML file")
	}

	var root mmlImportRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return apperror.Wrap(err, "IMPORT_MML_PARSE_FAILED", 400, "failed to parse MML XML")
	}

	hintMap := make(map[string]string, len(root.Hints))
	for _, h := range root.Hints {
		if h.Id != "" {
			hintMap[h.Id] = h.Info
		}
	}

	// Version replace: drop existing MmlSet/Command/Param for this version+license.
	if version != "" {
		if delErr := s.deleteMmlByVersionInternal(version, licenseId); delErr != nil {
			return delErr
		}
	}

	for i := range root.Mmls {
		folder := &root.Mmls[i]
		if folder.Type == "mml" {
			s.insertMMLSet(hintMap, folder, nil, licenseId, version)
		}
	}
	return nil
}

// insertMMLSet creates (or reuses) a folder node and recurses into its commands
// and sub-folders, 对齐 Java insertMMLSet.
func (s *service) insertMMLSet(hintMap map[string]string, folder *mmlFolder, parentId *int, licenseId int, version string) {
	setId := 0
	existing, err := s.repo.FindMmlSetByParentIdAndName(parentId, folder.Name, licenseId)
	if err != nil {
		logger.Errorf("insertMMLSet: failed to resolve existing set %q: %v", folder.Name, err)
		return
	}
	reused := false
	for _, e := range existing {
		if version == "" && e.Version == nil {
			setId = e.Id
			reused = true
			break
		}
		if version != "" && e.Version != nil && *e.Version == version {
			setId = e.Id
			reused = true
			break
		}
	}
	if !reused {
		set := &MmlSet{Name: mmlStrPtr(folder.Name), ParentId: parentId, LicenseId: &licenseId}
		if version != "" {
			set.Version = mmlStrPtr(version)
		}
		if err := s.repo.CreateMmlSet(set); err != nil {
			logger.Errorf("insertMMLSet: failed to create set %q: %v", folder.Name, err)
			return
		}
		setId = set.Id
	}

	// Commands under this folder.
	for i := range folder.ParaObjs {
		s.parseCommand(setId, &folder.ParaObjs[i], hintMap)
	}
	// Nested sub-folders.
	for i := range folder.Children {
		childId := setId
		s.insertMMLSet(hintMap, &folder.Children[i], &childId, licenseId, version)
	}
}

// parseCommand persists a MmlCommand and its params, 对齐 Java parseCommand.
func (s *service) parseCommand(setId int, po *mmlParaObj, hintMap map[string]string) {
	cmd := &MmlCommand{
		Command:   mmlStrPtr(po.CmdLabel),
		Name:      mmlStrPtr(po.Name),
		HelpFile:  mmlStrPtr(po.HelpFile),
		MmlSetId:  &setId,
	}
	if po.CmdType != "" {
		cmd.Type = mmlStrPtr(po.CmdType)
	}
	if po.Hint != "" {
		if info, ok := hintMap[po.Hint]; ok {
			cmd.Hint = mmlStrPtr(info)
		}
	}
	if err := s.repo.CreateMmlCommand(cmd); err != nil {
		logger.Errorf("parseCommand: failed to create command %q: %v", po.CmdLabel, err)
		return
	}
	s.generateCommandParam(cmd.Id, po)
}

// generateCommandParam persists MmlCommandParam rows for a command, 对齐 Java
// generateCommandParam (necessity/type/writable/option/range/od + enum default remap).
func (s *service) generateCommandParam(commandId int, po *mmlParaObj) {
	od := 1
	for i := range po.Params {
		p := &po.Params[i]
		param := &MmlCommandParam{
			MmlCommandId: &commandId,
			Name:         mmlStrPtr(p.Name),
			Parameter:    mmlStrPtr(p.Id),
			Od:           od,
			Necessity:    strings.EqualFold(p.Attr, "Must"),
			Type:         mmlStrPtr(p.Type),
			Relate:       mmlStrPtr(p.Relate),
		}
		if strings.EqualFold(p.Writable, "true") {
			param.Writable = true
		}

		defaultVal := p.Default
		if p.Type == "enum" {
			if p.Option != "" {
				param.Option = mmlStrPtr(p.Option)
			}
			// enum default remap: option "k=v,k2=v2" -> defaultValue(v) becomes k
			if p.Default != "" {
				if mapped, ok := mmlEnumValueToKey(p.Option, p.Default); ok {
					defaultVal = mapped
				} else {
					defaultVal = ""
				}
			}
		} else if p.Type == "checkbox" {
			if p.Option != "" {
				param.Option = mmlStrPtr(p.Option)
			}
		} else if p.Range != "" {
			param.ValueRange = mmlStrPtr(p.Range)
		}
		if defaultVal != "" {
			param.DefaultValue = mmlStrPtr(defaultVal)
		}

		if err := s.repo.CreateMmlCommandParam(param); err != nil {
			logger.Errorf("generateCommandParam: failed to create param %q: %v", p.Name, err)
		}
		od++
	}
}

// mmlEnumValueToKey parses an "k=v,k2=v2" option string and returns (key, true)
// for the mapping whose value equals the given default, 对齐 Java enum remap.
func mmlEnumValueToKey(option, defaultValue string) (string, bool) {
	for _, kv := range strings.Split(option, ",") {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) == 2 && pair[1] == defaultValue {
			return pair[0], true
		}
	}
	return "", false
}

// GetMmlVersions returns distinct versions for the license, 对齐 Java getMmlVersions.
func (s *service) GetMmlVersions(licenseId int) ([]string, error) {
	return s.repo.FindMmlVersions(licenseId)
}

// GetMmlCommandsByVersion returns the full command tree for a specific version,
// 对齐 Java getMmlCommandsByVersion.
func (s *service) GetMmlCommandsByVersion(version string, licenseId int) (*MmlVersionInfoVO, error) {
	sets, err := s.repo.FindMmlSetsByVersionAndLicense(version, licenseId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	vo := &MmlVersionInfoVO{Version: version, ImportTime: mmlTimePtr(now)}
	nodes := make([]MmlSetVo, 0, len(sets))
	for _, set := range sets {
		nodes = append(nodes, s.buildChildren(set))
	}
	vo.Commands = removeDuplicateNodes(nodes)
	return vo, nil
}

// GetMmlCommandTree returns the command tree for the CURRENT (max) version,
// 对齐 Java getMmlCommand (versions reverse-sorted, first = current).
func (s *service) GetMmlCommandTree(licenseId int) ([]MmlSetVo, error) {
	versions, err := s.repo.FindMmlVersions(licenseId)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return []MmlSetVo{}, nil
	}
	current := versions[0]
	for _, v := range versions[1:] {
		if v > current {
			current = v
		}
	}
	sets, err := s.repo.FindMmlSetsByVersionAndLicense(current, licenseId)
	if err != nil {
		return nil, err
	}
	nodes := make([]MmlSetVo, 0, len(sets))
	for _, set := range sets {
		nodes = append(nodes, s.buildChildren(set))
	}
	return removeDuplicateNodes(nodes), nil
}

// DeleteMmlByVersion cascade-deletes all MmlSet/Command/Param for a version,
// 对齐 Java deleteMmlByVersion.
func (s *service) DeleteMmlByVersion(version string, licenseId int) error {
	return s.deleteMmlByVersionInternal(version, licenseId)
}

// deleteMmlByVersionInternal is the shared cascade-delete used by both the
// explicit delete endpoint and the import version-replace path.
func (s *service) deleteMmlByVersionInternal(version string, licenseId int) error {
	sets, err := s.repo.FindMmlSetsByVersionAndLicense(version, licenseId)
	if err != nil {
		return apperror.Wrap(err, "DELETE_MML_QUERY_FAILED", 500, "failed to query MML sets")
	}
	if len(sets) == 0 {
		return apperror.ErrNotFound.WithMessage("no MML sets found for the specified version")
	}
	setIds := make([]int, 0, len(sets))
	for _, set := range sets {
		setIds = append(setIds, set.Id)
	}
	cmds, err := s.repo.FindMmlCommandsBySetIds(setIds)
	if err != nil {
		return apperror.Wrap(err, "DELETE_MML_QUERY_FAILED", 500, "failed to query MML commands")
	}
	cmdIds := make([]int, 0, len(cmds))
	for _, c := range cmds {
		cmdIds = append(cmdIds, c.Id)
	}
	params, err := s.repo.FindMmlCommandParamsByCommandIds(cmdIds)
	if err != nil {
		return apperror.Wrap(err, "DELETE_MML_QUERY_FAILED", 500, "failed to query MML command params")
	}
	paramIds := make([]int, 0, len(params))
	for _, p := range params {
		paramIds = append(paramIds, p.Id)
	}

	if err := s.repo.DeleteMmlCommandParamsByIds(paramIds); err != nil {
		return apperror.Wrap(err, "DELETE_MML_FAILED", 500, "failed to delete MML command params")
	}
	if err := s.repo.DeleteMmlCommandsByIds(cmdIds); err != nil {
		return apperror.Wrap(err, "DELETE_MML_FAILED", 500, "failed to delete MML commands")
	}
	if err := s.repo.DeleteMmlSetsByIds(setIds); err != nil {
		return apperror.Wrap(err, "DELETE_MML_FAILED", 500, "failed to delete MML sets")
	}
	return nil
}

// buildChildren recursively builds the MmlSetVo tree for a folder, 对齐 Java
// generateChildren (sub-folders + commands as mixed children).
func (s *service) buildChildren(set MmlSet) MmlSetVo {
	vo := MmlSetVo{Name: mmlDerefStr(set.Name)}
	children, err := s.repo.FindChildMmlSets(set.Id)
	if err != nil {
		logger.Errorf("buildChildren: failed to query child sets for %d: %v", set.Id, err)
		return vo
	}
	for _, child := range children {
		vo.Children = append(vo.Children, s.buildChildren(child))
	}
	cmds, err := s.repo.FindMmlCommands(set.Id)
	if err != nil {
		logger.Errorf("buildChildren: failed to query commands for %d: %v", set.Id, err)
		return vo
	}
	for _, c := range cmds {
		node := MmlSetVo{Name: mmlDerefStr(c.Name), IsCommand: true, CommandId: &c.Id}
		if c.Hint != nil {
			node.Hint = c.Hint
		}
		if c.HelpFile != nil {
			node.HelpFile = c.HelpFile
		}
		vo.Children = append(vo.Children, node)
	}
	return vo
}

// removeDuplicateNodes dedups the command tree by node identity, 对齐 Java
// removeDuplicateNodes / removeDuplicateNodesAcrossTree.
func removeDuplicateNodes(nodes []MmlSetVo) []MmlSetVo {
	seen := make(map[string]bool)
	return removeDuplicateNodesAcrossTree(nodes, seen)
}

func removeDuplicateNodesAcrossTree(nodes []MmlSetVo, seen map[string]bool) []MmlSetVo {
	unique := make([]MmlSetVo, 0, len(nodes))
	for _, node := range nodes {
		var nodeId string
		if node.CommandId != nil {
			if node.IsCommand {
				nodeId = "CMD:" + strconv.Itoa(*node.CommandId)
			} else {
				nodeId = "FOLDER:" + strconv.Itoa(*node.CommandId)
			}
		} else {
			nodeId = "NAME:" + node.Name + "|IS_CMD:" + strconv.FormatBool(node.IsCommand)
		}
		if !seen[nodeId] {
			seen[nodeId] = true
			if len(node.Children) > 0 {
				node.Children = removeDuplicateNodesAcrossTree(node.Children, seen)
			}
			unique = append(unique, node)
		}
	}
	return unique
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// ImportMML handles POST /mml/import (multipart file + version form field).
func (h *Handler) ImportMML(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.Error(c, 400, "file is required")
		return
	}
	version := c.PostForm("version")
	licenseId := middleware.GetLicenseId(c)

	f, err := fileHeader.Open()
	if err != nil {
		utils.Error(c, 500, "failed to open uploaded file")
		return
	}
	defer f.Close()

	if err := h.svc.ImportMMLAndParameter(f, version, licenseId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, gin.H{"message": "MML imported", "version": version})
}

// GetMmlVersions handles GET /mml/versions.
func (h *Handler) GetMmlVersions(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)
	versions, err := h.svc.GetMmlVersions(licenseId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, versions)
}

// GetMmlCommandsByVersion handles GET /mml/commands-by-version?version=.
func (h *Handler) GetMmlCommandsByVersion(c *gin.Context) {
	version := c.Query("version")
	if version == "" {
		utils.Error(c, 400, "version is required")
		return
	}
	licenseId := middleware.GetLicenseId(c)
	vo, err := h.svc.GetMmlCommandsByVersion(version, licenseId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, vo)
}

// GetMmlCommandTree handles GET /mml/command-tree (current version tree).
func (h *Handler) GetMmlCommandTree(c *gin.Context) {
	licenseId := middleware.GetLicenseId(c)
	tree, err := h.svc.GetMmlCommandTree(licenseId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, tree)
}

// DeleteMmlByVersion handles DELETE /mml/version?version=.
func (h *Handler) DeleteMmlByVersion(c *gin.Context) {
	version := c.Query("version")
	if version == "" {
		utils.Error(c, 400, "version is required")
		return
	}
	licenseId := middleware.GetLicenseId(c)
	if err := h.svc.DeleteMmlByVersion(version, licenseId); err != nil {
		utils.HandleError(c, err)
		return
	}
	utils.Success(c, nil)
}
