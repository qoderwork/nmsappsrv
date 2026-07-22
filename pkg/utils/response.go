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

// StatusCode writes HTTP 200 with an arbitrary numeric code in the response
// body. Used for Java-compatible 100xx login error codes (10047, 10048,
// 10296, …) which must appear in JSON.code while the transport status stays
// 200 OK — mirrors Java global exception handler behaviour.
func StatusCode(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithExtra returns an error envelope with the extra fields placed inside
// the "data" key, keeping the shape consistent with the standard Response:
// {"code":400, "message":"captcha required", "data":{"required":true}}
func ErrorWithExtra(c *gin.Context, code int, message string, extra map[string]interface{}) {
	c.JSON(code, Response{
		Code:    code,
		Message: message,
		Data:    extra,
	})
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
