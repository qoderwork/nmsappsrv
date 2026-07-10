package baserepo

import (
	"gorm.io/gorm"
)

// BaseRepository provides generic CRUD operations for any GORM model.
// T is the model type, PK is the primary key type (must be comparable).
type BaseRepository[T any, PK comparable] struct {
	DB       *gorm.DB
	PKColumn string
}

// New creates a BaseRepository. pkColumn defaults to "id" if empty.
func New[T any, PK comparable](db *gorm.DB, pkColumn string) *BaseRepository[T, PK] {
	if pkColumn == "" {
		pkColumn = "id"
	}
	return &BaseRepository[T, PK]{DB: db, PKColumn: pkColumn}
}

// PageResult encapsulates paginated query results.
type PageResult[T any] struct {
	Items []T
	Total int64
}

// ---------------------------------------------------------------------------
// Core CRUD
// ---------------------------------------------------------------------------

// Create inserts a new entity.
func (r *BaseRepository[T, PK]) Create(entity *T) error {
	return r.DB.Create(entity).Error
}

// Save updates an existing entity (full update).
func (r *BaseRepository[T, PK]) Save(entity *T) error {
	return r.DB.Save(entity).Error
}

// FindByID returns a single entity by primary key.
func (r *BaseRepository[T, PK]) FindByID(id PK) (*T, error) {
	var entity T
	if err := r.DB.Where(r.PKColumn+" = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// DeleteByID hard-deletes an entity by primary key.
func (r *BaseRepository[T, PK]) DeleteByID(id PK) error {
	var entity T
	return r.DB.Where(r.PKColumn+" = ?", id).Delete(&entity).Error
}

// DeleteByIDs hard-deletes multiple entities by primary keys.
func (r *BaseRepository[T, PK]) DeleteByIDs(ids []PK) error {
	var entity T
	return r.DB.Where(r.PKColumn+" IN ?", ids).Delete(&entity).Error
}

// SoftDelete sets deleted=true on an entity by primary key.
func (r *BaseRepository[T, PK]) SoftDelete(id PK) error {
	var entity T
	return r.DB.Model(&entity).Where(r.PKColumn+" = ?", id).Update("deleted", true).Error
}

// UpdateFields updates specific fields on an entity by primary key.
func (r *BaseRepository[T, PK]) UpdateFields(id PK, fields map[string]interface{}) error {
	var entity T
	return r.DB.Model(&entity).Where(r.PKColumn+" = ?", id).Updates(fields).Error
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// FindAll returns all entities matching the given query.
// Usage: base.FindAll(db.Where("status = ?", "active"))
func (r *BaseRepository[T, PK]) FindAll(query *gorm.DB) ([]T, error) {
	var items []T
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Count returns the count of entities matching the given query.
func (r *BaseRepository[T, PK]) Count(query *gorm.DB) (int64, error) {
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// FindPage returns offset/limit paginated results with total count.
// baseQuery should be a pre-built *gorm.DB with WHERE clauses.
// orderCol is the ORDER BY expression, e.g. "id DESC".
func (r *BaseRepository[T, PK]) FindPage(baseQuery *gorm.DB, orderCol string, offset, limit int) (*PageResult[T], error) {
	var items []T
	var total int64

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := baseQuery.Order(orderCol).Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return &PageResult[T]{Items: items, Total: total}, nil
}
