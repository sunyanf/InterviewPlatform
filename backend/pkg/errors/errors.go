package errors

import (
	"fmt"
	"net/http"
)

// AppError 应用统一错误类型
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New 创建业务错误
func New(code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

// Wrap 包装底层错误
func Wrap(code, message string, status int, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
		Err:     err,
	}
}

// 通用错误码
var (
	ErrBadRequest          = New("BAD_REQUEST", "请求参数错误", http.StatusBadRequest)
	ErrUnauthorized        = New("UNAUTHORIZED", "未授权", http.StatusUnauthorized)
	ErrForbidden           = New("FORBIDDEN", "无权限", http.StatusForbidden)
	ErrNotFound            = New("NOT_FOUND", "资源不存在", http.StatusNotFound)
	ErrConflict            = New("CONFLICT", "资源冲突", http.StatusConflict)
	ErrInternalServerError = New("INTERNAL_SERVER_ERROR", "服务器内部错误", http.StatusInternalServerError)

	// 认证相关
	ErrInvalidCredentials = New("INVALID_CREDENTIALS", "邮箱或密码错误", http.StatusUnauthorized)
	ErrEmailAlreadyExists = New("EMAIL_ALREADY_EXISTS", "邮箱已被注册", http.StatusConflict)
	ErrInvalidToken       = New("INVALID_TOKEN", "Token 无效或已过期", http.StatusUnauthorized)
)
