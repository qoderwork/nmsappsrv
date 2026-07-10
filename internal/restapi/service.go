package restapi

import (
	"fmt"
	"strings"
	"time"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ============================
// Helper functions
// ============================

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefIntPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func derefInt64Ptr(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// generateLinkHeader generates RFC 5988 Link headers for offset-based pagination
func generateLinkHeader(baseUrl string, offset, limit, total int) string {
	var links []string

	// next
	if offset+limit < total {
		nextOffset := offset + limit
		links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"next\"", baseUrl, nextOffset, limit))
	}

	// prev
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"prev\"", baseUrl, prevOffset, limit))
	}

	// first
	links = append(links, fmt.Sprintf("<%s?offset=0&limit=%d>; rel=\"first\"", baseUrl, limit))

	// last
	lastOffset := 0
	if total > 0 {
		lastOffset = ((total - 1) / limit) * limit
	}
	links = append(links, fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"last\"", baseUrl, lastOffset, limit))

	return strings.Join(links, ", ")
}
