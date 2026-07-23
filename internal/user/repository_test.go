package user

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupMockDB creates a sqlmock-backed *gorm.DB for repository tests.
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "dummy:dummy@tcp(127.0.0.1:3306)/dummy",
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm DB: %v", err)
	}

	return gormDB, mock
}

// ---------------------------------------------------------------------------
// Tests: FindUserByUsername
// ---------------------------------------------------------------------------

func TestRepository_FindUserByUsername(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewRepository(db)

	t.Run("returns user when found", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "username", "password"}).
			AddRow(1, "alice", "hashed_pw_alice")

		// Query: WHERE username = ? AND (deleted IS NULL OR deleted = ?) + ORDER BY + LIMIT 1
		// Parameters in GORM expected order: username, deleted=false, limit=1 (3 args total)
		mock.ExpectQuery(`SELECT \* FROM .+sys_user.+ WHERE username = \? .+deleted`).
			WithArgs("alice", false, 1).
			WillReturnRows(rows)

		u, err := repo.FindUserByUsername("alice")
		assert.NoError(t, err)
		require.NotNil(t, u)
		assert.Equal(t, 1, u.Id)
		assert.Equal(t, "alice", *u.Username)
		assert.Equal(t, "hashed_pw_alice", *u.Password)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT \* FROM .+sys_user.+ WHERE username = \? .+deleted`).
			WithArgs("ghost", false, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		u, err := repo.FindUserByUsername("ghost")
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// Tests: FindUserByID
// ---------------------------------------------------------------------------

func TestRepository_FindUserByID(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewRepository(db)

	t.Run("returns user when found", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "username", "email"}).
			AddRow(42, "bob", "bob@example.com")

		mock.ExpectQuery(`SELECT \* FROM .+sys_user.+ WHERE id = \?`).
			WithArgs(42, 1).
			WillReturnRows(rows)

		u, err := repo.FindUserByID(42)
		assert.NoError(t, err)
		require.NotNil(t, u)
		assert.Equal(t, 42, u.Id)
		assert.Equal(t, "bob", *u.Username)
		assert.Equal(t, "bob@example.com", *u.Email)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT \* FROM .+sys_user.+ WHERE id = \?`).
			WithArgs(999, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		u, err := repo.FindUserByID(999)
		assert.Error(t, err)
		assert.Nil(t, u)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// Tests: CreateUser
// ---------------------------------------------------------------------------

func TestRepository_CreateUser(t *testing.T) {
	t.Run("inserts user successfully", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewRepository(db)

		// GORM wraps Create in a transaction.
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO .+sys_user.+`).
			WillReturnResult(sqlmock.NewResult(10, 1))
		mock.ExpectCommit()

		username := "newuser"
		password := "hashed_pw"
		u := &SysUser{
			Username: &username,
			Password: &password,
		}

		err := repo.CreateUser(u)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on db failure", func(t *testing.T) {
		db, mock := setupMockDB(t)
		repo := NewRepository(db)

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO .+sys_user.+`).
			WillReturnError(assert.AnError)
		mock.ExpectRollback()

		u := &SysUser{Username: strPtr("fail")}
		err := repo.CreateUser(u)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
