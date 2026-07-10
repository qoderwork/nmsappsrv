package alarm

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMockDB creates a sqlmock and a *gorm.DB backed by it for testing.
func newMockDB(t *testing.T) (sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm DB: %v", err)
	}

	return mock, gormDB
}

func TestRepository_FindAlarmByID(t *testing.T) {
	mock, db := newMockDB(t)
	repo := NewRepository(db)

	now := time.Now().Truncate(time.Second)
	severity := "CRITICAL"

	rows := sqlmock.NewRows([]string{"id", "severity", "event_time"}).
		AddRow(1, severity, now)

	mock.ExpectQuery(`SELECT \* FROM .alarm. WHERE id = \?`).
		WithArgs(1, 1).
		WillReturnRows(rows)

	alarm, err := repo.FindAlarmByID(1)

	assert.NoError(t, err)
	require.NotNil(t, alarm)
	assert.Equal(t, int64(1), alarm.Id)
	assert.NotNil(t, alarm.Severity)
	assert.Equal(t, severity, *alarm.Severity)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindAlarmByID_NotFound(t *testing.T) {
	mock, db := newMockDB(t)
	repo := NewRepository(db)

	mock.ExpectQuery(`SELECT \* FROM .alarm. WHERE id = \?`).
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	alarm, err := repo.FindAlarmByID(999)

	assert.Error(t, err)
	assert.Nil(t, alarm)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_CreateAlarm(t *testing.T) {
	mock, db := newMockDB(t)
	repo := NewRepository(db)

	severity := "MAJOR"
	source := "BS-001"
	a := &Alarm{
		Severity:    &severity,
		AlarmSource: &source,
	}

	// GORM v2 wraps Create() in a transaction (BEGIN -> INSERT -> COMMIT).
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO .alarm.`).
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectCommit()

	err := repo.CreateAlarm(a)

	assert.NoError(t, err)
	assert.Equal(t, int64(42), a.Id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ClearAlarm(t *testing.T) {
	mock, db := newMockDB(t)
	repo := NewRepository(db)

	clearedTime := time.Now().Truncate(time.Second)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .alarm. SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.ClearAlarm(1, clearedTime)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ClearAlarm_NoRowsAffected(t *testing.T) {
	mock, db := newMockDB(t)
	repo := NewRepository(db)

	clearedTime := time.Now().Truncate(time.Second)

	// Return 0 rows affected -- GORM Updates() does not error on 0 rows.
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE .alarm. SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.ClearAlarm(999, clearedTime)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
