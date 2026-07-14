package cbsd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"nmsappsrv/pkg/baserepo"
)

// ----- test helpers -----------------------------------------------------

func intPtr(i int) *int       { return &i }
func int64Ptr(i int64) *int64 { return &i }
func float64Ptr(f float64) *float64 { return &f }

// fakeRepo is an in-memory Repository used to exercise the SAS state-machine
// methods (Grant / Relinquishment / SasHeartbeat / MaintainOperationStates)
// without a database or a real SAS endpoint. Only the methods those flows
// touch are functional; the rest are stubs returning zero values.
type fakeRepo struct {
	byID     map[string]*CbsdInfo
	byStates []CbsdInfo
	configs  []SasConfig
	logs     []CbrsLog
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byID: map[string]*CbsdInfo{}} }

func (f *fakeRepo) Create(entity *CbsdInfo) error { f.byID[entity.Id] = entity; return nil }
func (f *fakeRepo) Save(entity *CbsdInfo) error   { f.byID[entity.Id] = entity; return nil }
func (f *fakeRepo) FindByID(id string) (*CbsdInfo, error) {
	if e, ok := f.byID[id]; ok {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeRepo) DeleteByID(id string) error      { delete(f.byID, id); return nil }
func (f *fakeRepo) DeleteByIDs(ids []string) error { return nil }
func (f *fakeRepo) SoftDelete(id string) error      { return nil }
func (f *fakeRepo) UpdateFields(id string, fields map[string]interface{}) error {
	if e, ok := f.byID[id]; ok {
		if v, ok := fields["operation_state"]; ok {
			if s, ok := v.(string); ok {
				e.OperationState = &s
			}
		}
		if v, ok := fields["grant_id"]; ok {
			if s, ok := v.(string); ok {
				e.GrantID = &s
			}
		}
		if _, ok := fields["grant_expire_time"]; ok {
			e.GrantExpireTime = nil
		}
		if _, ok := fields["transmit_expire_time"]; ok {
			e.TransmitExpireTime = nil
		}
	}
	return nil
}
func (f *fakeRepo) FindAll(query *gorm.DB) ([]CbsdInfo, error) { return nil, nil }
func (f *fakeRepo) Count(query *gorm.DB) (int64, error)        { return 0, nil }
func (f *fakeRepo) FindPage(q *gorm.DB, o string, off, lim int) (*baserepo.PageResult[CbsdInfo], error) {
	return nil, nil
}
func (f *fakeRepo) FindCbsdInfos(lic int, off, lim int) ([]CbsdInfo, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) FindCbsdInfoBySN(sn string, lic int) (*CbsdInfo, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeRepo) FindCbrsLogs(cid, lt string, off, lim int) ([]CbrsLog, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) CreateCbrsLog(log *CbrsLog) error {
	f.logs = append(f.logs, *log)
	return nil
}
func (f *fakeRepo) CreateCertFileSendTask(t *CBSDCertFileSendTask) error { return nil }
func (f *fakeRepo) FindCertFileSendTasks(ten int, off, lim int) ([]CBSDCertFileSendTask, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) FindCbsdInfoByID(id string) (*CbsdInfo, error) {
	if e, ok := f.byID[id]; ok {
		return e, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeRepo) UpdateCbsdEnable(id string, enable bool) error { return nil }
func (f *fakeRepo) CountCbsdByStatus(lic int) ([]CbsdStatusCountItem, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreateCbsdInfos(infos []CbsdInfo) error { return nil }
func (f *fakeRepo) FindSasConfigs(lic int) ([]SasConfig, error) {
	var out []SasConfig
	for _, c := range f.configs {
		if c.LicenseId == lic {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeRepo) FindSasConfigByID(id int64) (*SasConfig, error) {
	for _, c := range f.configs {
		if int64(c.Id) == id {
			return &c, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeRepo) UpdateSasConfig(cfg *SasConfig) error { return nil }
func (f *fakeRepo) FindCbsdInfosByStates(states []string) ([]CbsdInfo, error) {
	return f.byStates, nil
}

// sasTestServer emulates a SAS endpoint. The returned *int controls the
// heartbeat responseCode so a single server can be reused across code tests.
func sasTestServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	hbCode := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/grant"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"grantId":            "G1",
				"grantExpireTime":    future,
				"transmitExpireTime": future,
			})
		case strings.HasSuffix(r.URL.Path, "/heartbeat"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": []interface{}{
					map[string]interface{}{"response": map[string]interface{}{"responseCode": float64(hbCode)}},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/relinquishment"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response": []interface{}{
					map[string]interface{}{"response": map[string]interface{}{"responseCode": float64(0)}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hbCode
}

// ----- Grant / Relinquishment / SasHeartbeat ----------------------------

func TestGrantSetsGranted(t *testing.T) {
	srv, _ := sasTestServer(t)
	repo := newFakeRepo()
	repo.configs = []SasConfig{{LicenseId: 1, SasUrl: srv.URL, Enabled: true}}
	repo.byID["c1"] = &CbsdInfo{
		Id: "c1", CbsdID: strPtr("CBSD-1"), LicenseId: intPtr(1),
		LowFrequency: int64Ptr(3550), HighFrequency: int64Ptr(3570), MaxEirp: float64Ptr(30),
	}
	svc := newService(repo)
	if _, err := svc.Grant("c1", &GrantRequest{LowFrequency: 3550, HighFrequency: 3570, MaxEirp: 30}); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	e := repo.byID["c1"]
	if derefString(e.OperationState) != OpStateGranted {
		t.Fatalf("state=%v", derefString(e.OperationState))
	}
	if derefString(e.GrantID) != "G1" {
		t.Fatalf("grantID=%v", derefString(e.GrantID))
	}
	if e.GrantExpireTime == nil || e.TransmitExpireTime == nil {
		t.Fatalf("expire times not set: g=%v t=%v", e.GrantExpireTime, e.TransmitExpireTime)
	}
}

func TestRelinquishmentResets(t *testing.T) {
	srv, _ := sasTestServer(t)
	repo := newFakeRepo()
	repo.configs = []SasConfig{{LicenseId: 1, SasUrl: srv.URL, Enabled: true}}
	repo.byID["c1"] = &CbsdInfo{
		Id: "c1", CbsdID: strPtr("CBSD-1"), LicenseId: intPtr(1),
		OperationState:    strPtr(OpStateGranted),
		GrantID:           strPtr("G0"),
		GrantExpireTime:   strPtr("2099-01-01T00:00:00Z"),
		TransmitExpireTime: strPtr("2099-01-01T00:00:00Z"),
	}
	svc := newService(repo)
	if _, err := svc.Relinquishment("c1", &RelinquishmentRequest{GrantId: "G0"}); err != nil {
		t.Fatalf("Relinquishment: %v", err)
	}
	e := repo.byID["c1"]
	if derefString(e.OperationState) != OpStateRegistered {
		t.Fatalf("state=%v", derefString(e.OperationState))
	}
	if derefString(e.GrantID) != "" {
		t.Fatalf("grantID should be cleared, got %q", derefString(e.GrantID))
	}
	if e.GrantExpireTime != nil || e.TransmitExpireTime != nil {
		t.Fatalf("expire times should be cleared")
	}
}

func TestSasHeartbeatCodes(t *testing.T) {
	srv, hbCode := sasTestServer(t)
	repo := newFakeRepo()
	repo.configs = []SasConfig{{LicenseId: 1, SasUrl: srv.URL, Enabled: true}}
	newSvc := func() Service {
		repo.byID["c1"] = &CbsdInfo{
			Id: "c1", CbsdID: strPtr("CBSD-1"), LicenseId: intPtr(1),
			OperationState: strPtr(OpStateGranted), GrantID: strPtr("G0"),
		}
		return newService(repo)
	}

	*hbCode = 0
	if _, err := newSvc().SasHeartbeat("c1"); err != nil {
		t.Fatalf("heartbeat 0: %v", err)
	}
	if derefString(repo.byID["c1"].OperationState) != OpStateAuthorized {
		t.Fatalf("code0 state=%v", derefString(repo.byID["c1"].OperationState))
	}

	*hbCode = 501
	if _, err := newSvc().SasHeartbeat("c1"); err != nil {
		t.Fatalf("heartbeat 501: %v", err)
	}
	if derefString(repo.byID["c1"].OperationState) != OpStateSuspended {
		t.Fatalf("code501 state=%v", derefString(repo.byID["c1"].OperationState))
	}

	*hbCode = 500
	if _, err := newSvc().SasHeartbeat("c1"); err != nil {
		t.Fatalf("heartbeat 500: %v", err)
	}
	if derefString(repo.byID["c1"].OperationState) != OpStateRegistered {
		t.Fatalf("code500 state=%v", derefString(repo.byID["c1"].OperationState))
	}
}

// ----- MaintainOperationStates ------------------------------------------

func TestMaintainGrantExpired(t *testing.T) {
	srv, _ := sasTestServer(t)
	repo := newFakeRepo()
	repo.configs = []SasConfig{{LicenseId: 1, SasUrl: srv.URL, Enabled: true}}
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	repo.byStates = []CbsdInfo{{
		Id: "c1", CbsdID: strPtr("CBSD-1"), LicenseId: intPtr(1),
		OperationState:    strPtr(OpStateGranted),
		GrantExpireTime:   strPtr(past),
		TransmitExpireTime: strPtr(past),
		LowFrequency:      int64Ptr(3550),
		HighFrequency:     int64Ptr(3570),
		MaxEirp:           float64Ptr(30),
		GrantID:           strPtr("G0"),
	}}
	repo.byID["c1"] = &repo.byStates[0]

	svc := newService(repo)
	n, err := svc.MaintainOperationStates(context.Background())
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 transition, got %d", n)
	}
	e := repo.byID["c1"]
	// re-grant should bring it back to GRANTED with a fresh grant id.
	if derefString(e.OperationState) != OpStateGranted {
		t.Fatalf("final state=%v", derefString(e.OperationState))
	}
	if derefString(e.GrantID) != "G1" {
		t.Fatalf("re-grant id=%v", derefString(e.GrantID))
	}
	if e.GrantExpireTime == nil {
		t.Fatalf("re-grant should set new expire time")
	}
	found := false
	for _, l := range repo.logs {
		if derefString(l.LogType) == "grant" && derefString(l.Status) == "expired" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no grant/expired log written: %+v", repo.logs)
	}
}

func TestMaintainTransmitExpired(t *testing.T) {
	srv, _ := sasTestServer(t)
	repo := newFakeRepo()
	repo.configs = []SasConfig{{LicenseId: 1, SasUrl: srv.URL, Enabled: true}}
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	repo.byStates = []CbsdInfo{{
		Id: "c1", CbsdID: strPtr("CBSD-1"), LicenseId: intPtr(1),
		OperationState:    strPtr(OpStateAuthorized),
		GrantExpireTime:   strPtr(future),
		TransmitExpireTime: strPtr(past),
		GrantID:           strPtr("G0"),
	}}
	repo.byID["c1"] = &repo.byStates[0]

	svc := newService(repo)
	n, err := svc.MaintainOperationStates(context.Background())
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 transition, got %d", n)
	}
	e := repo.byID["c1"]
	// re-heartbeat (code 0) should bring it back to AUTHORIZED.
	if derefString(e.OperationState) != OpStateAuthorized {
		t.Fatalf("final state=%v", derefString(e.OperationState))
	}
	found := false
	for _, l := range repo.logs {
		if derefString(l.LogType) == "heartbeat" && derefString(l.Status) == "transmit_expired" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no heartbeat/transmit_expired log: %+v", repo.logs)
	}
}

func TestMaintainNoTransition(t *testing.T) {
	srv, _ := sasTestServer(t)
	repo := newFakeRepo()
	repo.configs = []SasConfig{{LicenseId: 1, SasUrl: srv.URL, Enabled: true}}
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	repo.byStates = []CbsdInfo{{
		Id: "c1", CbsdID: strPtr("CBSD-1"), LicenseId: intPtr(1),
		OperationState:    strPtr(OpStateGranted),
		GrantExpireTime:   strPtr(future),
		TransmitExpireTime: strPtr(future),
	}}

	svc := newService(repo)
	n, err := svc.MaintainOperationStates(context.Background())
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 transitions, got %d", n)
	}
	if len(repo.logs) != 0 {
		t.Fatalf("no logs expected, got %d", len(repo.logs))
	}
}
