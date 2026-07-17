// Package authz implements the RBAC authorization layer for nmsappsrv,
// mirroring the Java nms-serv permission model (casbin policy engine).
//
// Java model (nms-serv RoleManagementServiceImpl):
//   - Permissions are dot-coded ids, e.g. "Alarm.ListAlarm", declared via the
//     @Permission annotation on @RestController methods.
//   - 4 built-in roles: admin (all perms), operator (treated as admin),
//     Maintenance (300 codes), Monitoring (25 codes).
//   - Custom roles grant permissions through the role_has_permission table.
//   - Enforcement: @PreAuthorize("hasAuthority('<id>')") + MyAccessDecisionManager.
//
// Go model:
//   - casbin enforcer with a minimal (sub, obj) model; obj = permission id,
//     sub = role name (as carried in the JWT role_names claim).
//   - Built-in roles are seeded from the extracted Java permission sets
//     (see seeds.go). admin/operator use a wildcard policy ("*").
//   - Custom role -> permission rows are loaded from role_has_permission.
//   - RequirePermission(permId) is the gin middleware equivalent of
//     @PreAuthorize("hasAuthority('<id>')").
package authz

import (
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nmsappsrv/internal/middleware"
	"nmsappsrv/pkg/utils"
)

// Built-in role identifiers (kept identical to Java so the default-role
// guard and any cross-reference stay aligned).
const (
	RoleAdminID       = "admin_id"
	RoleOperatorID    = "operator_id"
	RoleMaintenanceID = "Maintenance_id"
	RoleMonitoringID  = "Monitoring_id"

	RoleAdminName       = "admin"
	RoleOperatorName    = "operator"
	RoleMaintenanceName = "Maintenance"
	RoleMonitoringName  = "Monitoring"
)

// BuiltinRoleIDs is the set of immutable built-in role ids.
var BuiltinRoleIDs = map[string]bool{
	RoleAdminID:       true,
	RoleOperatorID:    true,
	RoleMaintenanceID: true,
	RoleMonitoringID:  true,
}

// casbin model: request (sub, obj); policy (sub, obj). The wildcard "*" in
// obj matches any permission id (used by admin/operator).
const modelText = `
[request_definition]
r = sub, obj

[policy_definition]
p = sub, obj

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && (p.obj == "*" || r.obj == p.obj)
`

// PermissionVO is a single permission entry in the registry / permission tree.
type PermissionVO struct {
	PermissionID string `json:"permission_id"`
	Module       string `json:"module"`
	Name         string `json:"name"`
}

// GetPermissionIdsForUserVO is the response of getPermissionIdsForUser.
type GetPermissionIdsForUserVO struct {
	Admin         bool     `json:"admin"`
	PermissionIDs []string `json:"permission_ids"`
}

var (
	mu       sync.RWMutex
	enf      *casbin.Enforcer
	registry []PermissionVO
	regSet   map[string]bool
)

// roleRow is a minimal projection of the `role` table used only for seeding
// built-in roles and reading custom-role names. Defined locally to avoid an
// import cycle with the user package (which imports this package).
type roleRow struct {
	Id          string  `gorm:"primaryKey;column:id;type:varchar(32)"`
	RoleName    *string `gorm:"column:name;type:varchar(255)"`
	DefaultRole *bool   `gorm:"column:default_role"`
}

func (roleRow) TableName() string { return "role" }

// rolePermRow is a minimal projection of the `role_has_permission` table.
type rolePermRow struct {
	RoleID       *string `gorm:"column:role_id;type:varchar(32)"`
	PermissionID *string `gorm:"column:permission_id;type:varchar(255)"`
}

func (rolePermRow) TableName() string { return "role_has_permission" }

func boolPtr(b bool) *bool { return &b }

// InitEnforcer builds the casbin enforcer, seeds built-in role policies and
// loads custom role permissions from the database, and computes the full
// permission registry. It is safe to call once at startup.
func InitEnforcer(db *gorm.DB) error {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return err
	}
	e, err := casbin.NewEnforcer(m, newMemAdapter())
	if err != nil {
		return err
	}
	mu.Lock()
	enf = e
	mu.Unlock()
	return Reload(db)
}

// Reload rebuilds all policies (built-in + custom) and the registry from the
// current database state. Call this after any role/permission mutation so the
// in-memory enforcer stays consistent with role_has_permission.
func Reload(db *gorm.DB) error {
	mu.Lock()
	defer mu.Unlock()
	if enf == nil {
		return nil
	}
	enf.ClearPolicy()

	// Built-in roles.
	policies := make([][]string, 0, len(BuiltinMaintainPerms)+len(BuiltinMonitorPerms)+2)
	policies = append(policies, []string{RoleAdminName, "*"})
	policies = append(policies, []string{RoleOperatorName, "*"})
	for _, p := range BuiltinMaintainPerms {
		policies = append(policies, []string{RoleMaintenanceName, p})
	}
	for _, p := range BuiltinMonitorPerms {
		policies = append(policies, []string{RoleMonitoringName, p})
	}
	if _, err := enf.AddPolicies(policies); err != nil {
		return err
	}

	// Registry base = built-in union.
	reg := make([]PermissionVO, 0, len(BuiltinRegistryBase))
	rs := make(map[string]bool, len(BuiltinRegistryBase))
	for _, id := range BuiltinRegistryBase {
		reg = append(reg, voFromID(id))
		rs[id] = true
	}

	// Custom roles: load role id->name and role_has_permission rows.
	if db != nil {
		var roles []roleRow
		if err := db.Find(&roles).Error; err == nil {
			nameByID := make(map[string]string, len(roles))
			for _, r := range roles {
				if r.RoleName != nil {
					nameByID[r.Id] = *r.RoleName
				}
			}
			var rps []rolePermRow
			if err := db.Find(&rps).Error; err == nil {
				for _, rp := range rps {
					if rp.RoleID == nil || rp.PermissionID == nil {
						continue
					}
					roleName, ok := nameByID[*rp.RoleID]
					if !ok || BuiltinRoleIDs[*rp.RoleID] {
						// Built-in role perms already seeded above.
						continue
					}
					if _, err := enf.AddPolicy(roleName, *rp.PermissionID); err != nil {
						return err
					}
					if !rs[*rp.PermissionID] {
						rs[*rp.PermissionID] = true
						reg = append(reg, voFromID(*rp.PermissionID))
					}
				}
			}
		}
	}

	registry = reg
	regSet = rs
	return nil
}

// voFromID derives a PermissionVO (module = first dot segment, name = rest)
// from a dot-coded permission id.
func voFromID(id string) PermissionVO {
	idx := strings.Index(id, ".")
	module, name := id, id
	if idx >= 0 {
		module = id[:idx]
		name = id[idx+1:]
	}
	return PermissionVO{PermissionID: id, Module: module, Name: name}
}

// Enforce reports whether any of the given role names is granted permId.
// admin/operator match any permission via the wildcard policy.
func Enforce(roleNames []string, permID string) bool {
	mu.RLock()
	e := enf
	mu.RUnlock()
	if e == nil {
		return false
	}
	for _, r := range roleNames {
		if allowed, _ := e.Enforce(r, permID); allowed {
			return true
		}
	}
	return false
}

// RequirePermission returns a gin middleware that aborts with 403 unless the
// caller's roles grant permID. This is the Go equivalent of Java's
// @PreAuthorize("hasAuthority('<id>')").
func RequirePermission(permID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleNames := middleware.GetRoleNames(c)
		if Enforce(roleNames, permID) {
			c.Next()
			return
		}
		utils.Error(c, 403, "forbidden: missing permission "+permID)
		c.Abort()
	}
}

// CurrentUserPermissionIDs returns the effective permission ids for a set of
// role names, mirroring Java getPermissionIdsForUser:
//   - admin/operator => the full registry (and Admin=true).
//   - others => the union of their granted permission ids.
func CurrentUserPermissionIDs(roleNames []string) *GetPermissionIdsForUserVO {
	vo := &GetPermissionIdsForUserVO{PermissionIDs: []string{}}
	isAdmin := false
	for _, r := range roleNames {
		if r == RoleAdminName || r == RoleOperatorName {
			isAdmin = true
		}
	}

	mu.RLock()
	e := enf
	reg := registry
	mu.RUnlock()

	if isAdmin {
		vo.Admin = true
		ids := make([]string, 0, len(reg))
		for _, p := range reg {
			ids = append(ids, p.PermissionID)
		}
		vo.PermissionIDs = ids
		return vo
	}

	set := make(map[string]bool)
	if e != nil {
		for _, r := range roleNames {
			ps, _ := e.GetFilteredPolicy(0, r)
			for _, p := range ps {
				if len(p) >= 2 && p[1] != "*" {
					set[p[1]] = true
				}
			}
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	vo.PermissionIDs = ids
	return vo
}

// PermissionRegistry returns all known permission ids (built-in union plus
// any custom permission referenced in role_has_permission), as a permission
// tree. Mirrors Java getAllPermissions()/getPermission().
func PermissionRegistry() []PermissionVO {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]PermissionVO, len(registry))
	copy(out, registry)
	return out
}

// HasPermission reports whether id is part of the known registry.
func HasPermission(id string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return regSet[id]
}

// SeedBuiltinRoles idempotently inserts the four built-in roles into the role
// table so they can be listed and assigned like any other role. It does not
// touch their permissions (those are enforced by the casbin wildcard/built-in
// policies, not by role_has_permission rows).
func SeedBuiltinRoles(db *gorm.DB) error {
	builtins := []struct {
		id   string
		name string
	}{
		{RoleAdminID, RoleAdminName},
		{RoleOperatorID, RoleOperatorName},
		{RoleMaintenanceID, RoleMaintenanceName},
		{RoleMonitoringID, RoleMonitoringName},
	}
	for _, b := range builtins {
		var cnt int64
		if err := db.Model(&roleRow{}).Where("id = ?", b.id).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			continue
		}
		name := b.name
		row := roleRow{Id: b.id, RoleName: &name, DefaultRole: boolPtr(true)}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
