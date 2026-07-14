package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSeedCounts(t *testing.T) {
	if len(BuiltinMonitorPerms) != 25 {
		t.Errorf("BuiltinMonitorPerms = %d, want 25", len(BuiltinMonitorPerms))
	}
	if len(BuiltinMaintainPerms) != 300 {
		t.Errorf("BuiltinMaintainPerms = %d, want 300", len(BuiltinMaintainPerms))
	}
	if len(BuiltinOperatorPerms) != 13 {
		t.Errorf("BuiltinOperatorPerms = %d, want 13", len(BuiltinOperatorPerms))
	}
	if len(BuiltinRegistryBase) != 294 {
		t.Errorf("BuiltinRegistryBase = %d, want 294", len(BuiltinRegistryBase))
	}
}

func TestInitAndEnforceBuiltin(t *testing.T) {
	if err := InitEnforcer(nil); err != nil {
		t.Fatalf("InitEnforcer(nil) error: %v", err)
	}

	// admin / operator match any permission (wildcard policy).
	if !Enforce([]string{"admin"}, "Any.Module.AnyPerm") {
		t.Error("admin should allow any permission")
	}
	if !Enforce([]string{"operator"}, "Any.Module.AnyPerm") {
		t.Error("operator should allow any permission")
	}

	// Maintenance: grants its 300-code set.
	if !Enforce([]string{"Maintenance"}, "Alarm.ConfirmAlarm") {
		t.Error("Maintenance should allow Alarm.ConfirmAlarm")
	}
	if !Enforce([]string{"Maintenance"}, "ZTP.ListZTPResults") {
		t.Error("Maintenance should allow ZTP.ListZTPResults")
	}
	if Enforce([]string{"Maintenance"}, "Some.Custom.Perm") {
		t.Error("Maintenance should NOT allow an unassigned permission")
	}

	// Monitoring: grants its 25-code set only.
	if !Enforce([]string{"Monitoring"}, "Alarm.ListAlarm") {
		t.Error("Monitoring should allow Alarm.ListAlarm")
	}
	if Enforce([]string{"Monitoring"}, "Alarm.ConfirmAlarm") {
		t.Error("Monitoring should NOT allow Alarm.ConfirmAlarm (not in monitor set)")
	}

	// Unknown role / no roles: deny.
	if Enforce([]string{"NoSuchRole"}, "Alarm.ListAlarm") {
		t.Error("unknown role must be denied")
	}
	if Enforce(nil, "Alarm.ListAlarm") {
		t.Error("nil roles must be denied")
	}
	if Enforce([]string{}, "Alarm.ListAlarm") {
		t.Error("empty roles must be denied")
	}
}

func TestCurrentUserPermissionIDs(t *testing.T) {
	if err := InitEnforcer(nil); err != nil {
		t.Fatalf("InitEnforcer(nil) error: %v", err)
	}

	// admin => full registry, Admin=true.
	vo := CurrentUserPermissionIDs([]string{"admin"})
	if !vo.Admin {
		t.Error("admin should report Admin=true")
	}
	if len(vo.PermissionIDs) != len(BuiltinRegistryBase) {
		t.Errorf("admin permission count = %d, want %d", len(vo.PermissionIDs), len(BuiltinRegistryBase))
	}

	// Monitoring => only its 25 codes, Admin=false.
	vo2 := CurrentUserPermissionIDs([]string{"Monitoring"})
	if vo2.Admin {
		t.Error("Monitoring should not report Admin=true")
	}
	if !contains(vo2.PermissionIDs, "Alarm.ListAlarm") {
		t.Error("Monitoring should include Alarm.ListAlarm")
	}
	if contains(vo2.PermissionIDs, "Alarm.ConfirmAlarm") {
		t.Error("Monitoring should NOT include Alarm.ConfirmAlarm")
	}
}

func TestRequirePermissionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := InitEnforcer(nil); err != nil {
		t.Fatalf("InitEnforcer(nil) error: %v", err)
	}

	cases := []struct {
		name      string
		roleNames []string
		perm      string
		wantAllow bool
	}{
		{"admin allowed", []string{"admin"}, "Alarm.ConfirmAlarm", true},
		{"monitoring allowed", []string{"Monitoring"}, "Alarm.ListAlarm", true},
		{"monitoring denied", []string{"Monitoring"}, "Alarm.ConfirmAlarm", false},
		{"no roles denied", nil, "Alarm.ListAlarm", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role_names", tc.roleNames)

			mw := RequirePermission(tc.perm)
			mw(c)

			if tc.wantAllow {
				if c.IsAborted() {
					t.Errorf("expected allowed (not aborted), got aborted; code=%d", w.Code)
				}
			} else {
				if !c.IsAborted() {
					t.Error("expected denied (aborted)")
				}
				if w.Code != http.StatusForbidden {
					t.Errorf("expected 403, got %d", w.Code)
				}
			}
		})
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
