package dto

// Page mirrors the Java Page VO (com.waveoss.common.vo.base.Page) exactly:
// same field names (pageNumber / pageSize / totalCount / orderName / orderType),
// same defaults, same MAX_PAGE_SIZE ceiling of 400.
const (
	DefaultPageNumber = 1
	DefaultPageSize   = 50
	MaxPageSize       = 400
)

type Page struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	TotalCount *int64 `json:"totalCount,omitempty"`
	OrderName  string `json:"orderName,omitempty"`
	OrderType  string `json:"orderType,omitempty"`
}

func (p *Page) Normalize() {
	if p.PageNumber <= 0 {
		p.PageNumber = DefaultPageNumber
	}
	if p.PageSize <= 0 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
}

// Offset returns the SQL OFFSET for this page.
func (p *Page) Offset() int {
	p.Normalize()
	return (p.PageNumber - 1) * p.PageSize
}

// Limit returns the SQL LIMIT for this page.
func (p *Page) Limit() int {
	p.Normalize()
	return p.PageSize
}
