package handlers

import (
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/util/errcode"
)

// APIResponse 统一 API 响应格式
type APIResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	ErrorCode string      `json:"error_code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// successResp returns a 200 response. Kept for backward compatibility.
func successResp(data interface{}) APIResponse {
	return APIResponse{Code: http.StatusOK, Message: "success", Data: data}
}

// errorResp returns a 500 response. Deprecated: use respondErr instead.
func errorResp(msg string) APIResponse {
	return APIResponse{Code: http.StatusInternalServerError, Message: msg}
}

// errorRespWithCode returns a custom-status response. Deprecated: use respondErr instead.
func errorRespWithCode(code int, msg string) APIResponse {
	return APIResponse{Code: code, Message: msg}
}

// respondOK is the canonical success response using the new errcode system.
func respondOK(c echo.Context, data interface{}) error {
	return errcode.RespondOK(c, data)
}

// respondErr is the canonical error response using structured error codes.
func respondErr(c echo.Context, code errcode.Code, detail string) error {
	return errcode.Respond(c, errcode.New(code, detail))
}

// respondErrWrap responds with a wrapped error.
func respondErrWrap(c echo.Context, code errcode.Code, detail string, cause error) error {
	return errcode.Respond(c, errcode.Wrap(code, detail, cause))
}

// respondInternal is a shorthand for INTERNAL_ERROR responses.
func respondInternal(c echo.Context, detail string, cause error) error {
	return errcode.Respond(c, errcode.Internal(detail, cause))
}

// logAndRespond logs and responds with an internal error.
func logAndRespond(c echo.Context, detail string, cause error) error {
	log.Error(detail, "error", cause)
	return respondInternal(c, detail, cause)
}
