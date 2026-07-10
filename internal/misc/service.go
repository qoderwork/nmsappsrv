package misc

import (
	"time"

	"gorm.io/gorm"
)

// Service contains the business logic for miscellaneous operations.
type Service struct {
	repo *Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) *Service {
	return &Service{repo: NewRepository(db)}
}

// ---------- helpers ----------

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
func ptrTime(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format(time.RFC3339)
}
func ptrTimePtr(p *time.Time) *time.Time { return p }
