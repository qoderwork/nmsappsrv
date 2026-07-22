package dto

// RequestDataDTO mirrors the Java generic envelope
// com.waveoss.common.dto.RequestDataDTO<T,D> exactly, with the same
// structural layout: {page, query, data, requestId}.
//
// T = query type (filter / search criteria)
// D = data  type (body payload for create / update operations)
type RequestDataDTO[T any, D any] struct {
	Page      Page   `json:"page"`
	Query     T      `json:"query"`
	Data      D      `json:"data"`
	RequestID string `json:"requestId,omitempty"`
}

// Normalize ensures the Page fields are valid and the request-id is extracted
// for tracing purposes.
func (r *RequestDataDTO[T, D]) Normalize() {
	r.Page.Normalize()
}
