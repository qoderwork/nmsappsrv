package corenet

import (
	"testing"
	"time"

	"nmsappsrv/pkg/baserepo"

	"gorm.io/gorm"
)

// mockRepository is a test double implementing Repository.
type mockRepository struct {
	deleteDataCalledWith int
	kpisCalledWith       struct {
		id    int
		start time.Time
		end   time.Time
	}
	logsCalledWith struct {
		id     int
		offset int
		limit  int
	}
	logResult []CoreNetworkOperationLog
	logTotal  int64
}

func (m *mockRepository) Create(*CoreNetwork) error                                       { return nil }
func (m *mockRepository) Save(*CoreNetwork) error                                         { return nil }
func (m *mockRepository) FindByID(int) (*CoreNetwork, error)                              { return nil, nil }
func (m *mockRepository) DeleteByID(int) error                                            { return nil }
func (m *mockRepository) DeleteByIDs([]int) error                                         { return nil }
func (m *mockRepository) SoftDelete(int) error                                            { return nil }
func (m *mockRepository) UpdateFields(int, map[string]interface{}) error                  { return nil }
func (m *mockRepository) FindAll(*gorm.DB) ([]CoreNetwork, error)                         { return nil, nil }
func (m *mockRepository) Count(*gorm.DB) (int64, error)                                   { return 0, nil }
func (m *mockRepository) FindPage(*gorm.DB, string, int, int) (*baserepo.PageResult[CoreNetwork], error) {
	return nil, nil
}
func (m *mockRepository) FindCoreNetworks(int) ([]CoreNetwork, error) { return nil, nil }
func (m *mockRepository) FindCoreNetworkData(int) (*CoreNetworkData, error) {
	return nil, nil
}
func (m *mockRepository) SaveCoreNetworkData(*CoreNetworkData) error { return nil }
func (m *mockRepository) DeleteCoreNetworkData(id int) error {
	m.deleteDataCalledWith = id
	return nil
}
func (m *mockRepository) FindCoreNetworkKpis(id int, start, end time.Time) ([]CoreNetworkKpi, error) {
	m.kpisCalledWith = struct {
		id    int
		start time.Time
		end   time.Time
	}{id, start, end}
	return nil, nil
}
func (m *mockRepository) FindCoreNetworkStatisticData(int, time.Time, time.Time) ([]CoreNetworkStatisticData, error) {
	return nil, nil
}
func (m *mockRepository) CreateOperationLog(*CoreNetworkOperationLog) error { return nil }
func (m *mockRepository) FindOperationLogs(id, offset, limit int) ([]CoreNetworkOperationLog, int64, error) {
	m.logsCalledWith = struct {
		id     int
		offset int
		limit  int
	}{id, offset, limit}
	return m.logResult, m.logTotal, nil
}

func TestDeleteCoreNetwork_CascadesToData(t *testing.T) {
	m := &mockRepository{}
	svc := newService(m)
	if err := svc.DeleteCoreNetwork(42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.deleteDataCalledWith != 42 {
		t.Fatalf("expected DeleteCoreNetworkData(42), got %d", m.deleteDataCalledWith)
	}
}

func TestListOperationLogs_DefaultPagination(t *testing.T) {
	m := &mockRepository{logResult: []CoreNetworkOperationLog{{}}, logTotal: 1}
	svc := newService(m)
	if _, _, err := svc.ListOperationLogs(7, 0, 0); err != nil { // page/pageSize < 1 -> defaults 1/20
		t.Fatalf("unexpected error: %v", err)
	}
	if m.logsCalledWith.offset != 0 || m.logsCalledWith.limit != 20 {
		t.Fatalf("expected default offset 0 limit 20, got offset=%d limit=%d", m.logsCalledWith.offset, m.logsCalledWith.limit)
	}
}

func TestGetCoreNetworkKpis_ParsesTimeAndForwards(t *testing.T) {
	m := &mockRepository{}
	svc := newService(m)
	if _, err := svc.GetCoreNetworkKpis(5, "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.kpisCalledWith.id != 5 {
		t.Fatalf("expected id 5, got %d", m.kpisCalledWith.id)
	}
	if m.kpisCalledWith.start.IsZero() || m.kpisCalledWith.end.IsZero() {
		t.Fatalf("expected parsed times, got start=%v end=%v", m.kpisCalledWith.start, m.kpisCalledWith.end)
	}
}
