package errcode

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIResponse is the standard JSON envelope for all API responses.
type APIResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
	Data      any    `json:"data,omitempty"`
}

// Respond writes a structured error response and logs internal errors.
func Respond(c echo.Context, err *AppError) error {
	if err == nil {
		return RespondOK(c, nil)
	}

	status := err.Code.HTTPStatus()
	resp := APIResponse{
		Code:      status,
		Message:   err.Message,
		ErrorCode: string(err.Code),
	}

	// Log internal errors with stack
	if err.Code == ErrInternal {
		slog.Error("internal error",
			"detail", err.Detail,
			"cause", err.Cause,
			"stack", err.Stack,
		)
	} else if err.Cause != nil {
		slog.Warn("app error",
			"code", err.Code,
			"detail", err.Detail,
			"cause", err.Cause,
		)
	}

	return c.JSON(status, resp)
}

// RespondOK writes a 200 success response.
func RespondOK(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})
}
