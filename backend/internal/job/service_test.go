package job

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	apperrors "ai-interview-platform/pkg/errors"
)

func newTestService() *Service {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	return NewService(nil, log)
}

func TestCreateJob_Validation(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name    string
		req     CreateJobRequest
		wantErr bool
	}{
		{
			name:    "empty title",
			req:     CreateJobRequest{CategoryID: "cat-1"},
			wantErr: true,
		},
		{
			name:    "empty category",
			req:     CreateJobRequest{Title: "Go 工程师"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateJob(context.Background(), tt.req)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// TestCreateJob_ContentRejected 任一字段命中违禁词即拒绝（400 CONTENT_REJECTED），且在落库前拦截
func TestCreateJob_ContentRejected(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name string
		req  CreateJobRequest
	}{
		{"首个字段 title 命中", CreateJobRequest{CategoryID: "cat-1", Title: "代考包过工程师"}},
		{"description 命中", CreateJobRequest{CategoryID: "cat-1", Title: "Go 工程师", Description: "可帮忙办证"}},
		{"source 命中", CreateJobRequest{CategoryID: "cat-1", Title: "Go 工程师", Source: "博彩平台"}},
		{"requirements 数组命中", CreateJobRequest{CategoryID: "cat-1", Title: "Go 工程师", Requirements: []string{"熟悉代考流程"}}},
		{"skills 数组命中", CreateJobRequest{CategoryID: "cat-1", Title: "Go 工程师", Skills: []string{"替考"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateJob(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected CONTENT_REJECTED, got nil")
			}
			var ae *apperrors.AppError
			if !errors.As(err, &ae) || ae.Code != "CONTENT_REJECTED" || ae.Status != 400 {
				t.Fatalf("got %v, want CONTENT_REJECTED 400", err)
			}
		})
	}
}
