package response

import (
	"encoding/json"
	"net/http"

	apperrors "ai-interview-platform/pkg/errors"
)

// Response 统一响应格式
type Response struct {
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应
func Success(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    "SUCCESS",
		Message: "ok",
		Data:    data,
	})
}

// Created 返回创建成功响应
func Created(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    "SUCCESS",
		Message: "created",
		Data:    data,
	})
}

// Accepted 返回已接受响应（异步任务已入队，客户端凭 task_id 轮询）
func Accepted(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    "ACCEPTED",
		Message: "accepted",
		Data:    data,
	})
}

// Error 返回错误响应
func Error(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	var appErr *apperrors.AppError
	if e, ok := err.(*apperrors.AppError); ok {
		appErr = e
	} else {
		appErr = apperrors.ErrInternalServerError
	}

	w.WriteHeader(appErr.Status)
	_ = json.NewEncoder(w).Encode(Response{
		Code:    appErr.Code,
		Message: appErr.Message,
	})
}
