package utils

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"nmsappsrv/pkg/apperror"
	"nmsappsrv/pkg/logger"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// pageData wraps list items with pagination metadata.
type pageData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "Success",
		Data:    data,
	})
}

// OK 是 Success 的别名
func OK(c *gin.Context, data interface{}) {
	Success(c, data)
}

func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithExtra returns an error envelope (same shape as Error) merged with
// extra fields. Used when the response must carry additional signals, e.g.
// {"required": true} on a captcha challenge.
func ErrorWithExtra(c *gin.Context, code int, message string, extra map[string]interface{}) {
	body := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	for k, v := range extra {
		body[k] = v
	}
	c.JSON(code, body)
}

func Paginated(c *gin.Context, data interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: "Success",
		Data: pageData{
			List:     data,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// HandleError handles errors in a unified way. If the error is an AppError,
// it extracts the status code and message. Otherwise, it logs the error and
// returns a generic 500 response to avoid leaking internal details.
func HandleError(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		Error(c, appErr.StatusCode, appErr.Message)
		return
	}
	// Unrecognized error — log full details, return generic message
	logger.Errorf("unhandled error in %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	Error(c, http.StatusInternalServerError, "internal server error")
}
