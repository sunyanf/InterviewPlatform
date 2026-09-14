package job

import (
	"context"
	"log/slog"
	"os"
	"testing"
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
