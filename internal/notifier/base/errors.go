package base

import "fmt"

// Error 通用通知错误
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("通知错误 [%s]: %s", e.Code, e.Message)
}

// IsError 判断错误是否为指定错误码的通知错误
func IsError(err error, code string) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}
