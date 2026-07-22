package device

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupMockDB creates a sqlmock-backed *gorm.DB suitable for repository tests.
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      mockDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	t.Cleanup(func() { mockDB.Close() })
	return gormDB, mock
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestRepository_FindByID(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	rows := sqlmock.NewRows([]string{"ne_neid", "serial_number", "device_name", "deleted"}).
		AddRow(1, "SN001", "Device1", false)

	mock.ExpectQuery("SELECT \\* FROM `cpe_element` WHERE ne_neid = .+ AND deleted = .+").
		WillReturnRows(rows)

	elem, err := repo.FindByID(1)
	assert.NoError(t, err)
	assert.NotNil(t, elem)
	assert.Equal(t, int64(1), elem.NeNeid)
	assert.Equal(t, "SN001", *elem.SerialNumber)
	assert.Equal(t, "Device1", *elem.DeviceName)
	assert.False(t, elem.Deleted)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	mock.ExpectQuery("SELECT \\* FROM `cpe_element` WHERE ne_neid = .+ AND deleted = .+").
		WillReturnError(gorm.ErrRecordNotFound)

	elem, err := repo.FindByID(999)
	assert.Error(t, err)
	assert.Nil(t, elem)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// FindPage
// ---------------------------------------------------------------------------

func TestRepository_FindPage(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	// GORM issues a COUNT query first, then the SELECT query.
	countRows := sqlmock.NewRows([]string{"count(*)"}).AddRow(2)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `cpe_element` WHERE deleted = .+ AND tenant_id = .+").
		WillReturnRows(countRows)

	dataRows := sqlmock.NewRows([]string{"ne_neid", "serial_number", "device_name", "deleted"}).
		AddRow(2, "SN002", "Device2", false).
		AddRow(1, "SN001", "Device1", false)
	mock.ExpectQuery("SELECT \\* FROM `cpe_element` WHERE deleted = .+ AND tenant_id = .+ ORDER BY ne_neid DESC").
		WillReturnRows(dataRows)

	elems, total, err := repo.FindPage(1, "", 0, 20)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, elems, 2)
	assert.Equal(t, int64(2), elems[0].NeNeid)
	assert.Equal(t, int64(1), elems[1].NeNeid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindPage_WithOffset(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	countRows := sqlmock.NewRows([]string{"count(*)"}).AddRow(50)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `cpe_element`").
		WillReturnRows(countRows)

	// Page 3 with pageSize 10 -> offset 20, limit 10
	dataRows := sqlmock.NewRows([]string{"ne_neid", "serial_number"}).
		AddRow(30, "SN030").
		AddRow(29, "SN029")
	mock.ExpectQuery("SELECT \\* FROM `cpe_element`").
		WillReturnRows(dataRows)

	elems, total, err := repo.FindPage(1, "", 20, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(50), total)
	assert.Len(t, elems, 2)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestRepository_Create(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `cpe_element`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectCommit()

	sn := "SN-NEW"
	name := "NewDevice"
	elem := &CpeElement{
		SerialNumber: &sn,
		DeviceName:   &name,
	}

	err := repo.Create(elem)
	assert.NoError(t, err)
	// GORM writes back the auto-increment primary key.
	assert.Equal(t, int64(100), elem.NeNeid)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// SoftDelete
// ---------------------------------------------------------------------------

func TestRepository_SoftDelete(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `cpe_element` SET `deleted`=.+ WHERE ne_neid = .+").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.SoftDelete(42)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_SoftDelete_NoMatch(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	repo := NewRepository(gormDB)

	// 0 rows affected -- device ID doesn't exist, but no error from SQL level.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `cpe_element` SET `deleted`=.+ WHERE ne_neid = .+").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.SoftDelete(99999)
	// SoftDelete doesn't check rows affected, so no error.
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}
