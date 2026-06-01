package alidns

import "fmt"

// APIError 阿里云 API 错误
type APIError struct {
	Code      string
	Message   string
	RequestID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("阿里云 API 错误 [%s]: %s (RequestID: %s)", e.Code, e.Message, e.RequestID)
}

// 常见错误码
var (
	ErrInvalidAccessKey      = &APIError{Code: "InvalidAccessKeyId.NotFound", Message: "AccessKey ID 不存在"}
	ErrSignatureDoesNotMatch = &APIError{Code: "SignatureDoesNotMatch", Message: "签名不匹配"}
	ErrDomainNotFound        = &APIError{Code: "DomainNotFound", Message: "域名不存在"}
	ErrDomainRecordNotFound  = &APIError{Code: "DomainRecordNotFound", Message: "域名记录不存在"}
	ErrQuotaExceeded         = &APIError{Code: "QuotaExceeded.DomainRecord", Message: "域名记录数量超限"}
)

// IsAPIError 判断是否为特定 API 错误
func IsAPIError(err error, code string) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == code
	}
	return false
}
