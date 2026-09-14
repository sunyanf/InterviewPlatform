package job

import (
	"context"
	"log/slog"

	apperrors "ai-interview-platform/pkg/errors"
)

// Service 岗位业务逻辑层
type Service struct {
	repo *Repository
	log  *slog.Logger
}

// NewService 创建岗位 Service
func NewService(repo *Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ListCategories 查询岗位分类列表
func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	list, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询岗位分类失败", 500, err)
	}
	return list, nil
}

// CreateJob 创建岗位
func (s *Service) CreateJob(ctx context.Context, req CreateJobRequest) (*Job, error) {
	if req.Title == "" {
		return nil, apperrors.New("INVALID_TITLE", "岗位标题不能为空", 400)
	}
	if req.CategoryID == "" {
		return nil, apperrors.New("INVALID_CATEGORY", "请选择岗位分类", 400)
	}

	j := &Job{
		CategoryID:   req.CategoryID,
		Title:        req.Title,
		Description:  req.Description,
		Requirements: req.Requirements,
		Skills:       req.Skills,
		Source:       req.Source,
		SourceURL:    req.SourceURL,
	}

	if j.Requirements == nil {
		j.Requirements = []string{}
	}
	if j.Skills == nil {
		j.Skills = []string{}
	}

	if err := s.repo.CreateJob(ctx, j); err != nil {
		return nil, apperrors.Wrap("INTERNAL_ERROR", "创建岗位失败", 500, err)
	}

	return j, nil
}

// GetJob 查询岗位详情
func (s *Service) GetJob(ctx context.Context, id string) (*Job, error) {
	if id == "" {
		return nil, apperrors.New("INVALID_ID", "岗位 ID 不能为空", 400)
	}

	j, err := s.repo.GetJobByID(ctx, id)
	if err != nil {
		s.log.Error("get job failed", "id", id, "error", err)
		return nil, apperrors.Wrap("INTERNAL_ERROR", "查询岗位失败", 500, err)
	}
	if j == nil {
		return nil, apperrors.ErrNotFound
	}
	return j, nil
}

// ListJobs 分页查询岗位列表
func (s *Service) ListJobs(ctx context.Context, req ListJobsRequest) ([]Job, int, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := s.repo.ListJobs(ctx, req.CategoryID, page, pageSize)
	if err != nil {
		s.log.Error("list jobs failed", "category_id", req.CategoryID, "error", err)
		return nil, 0, apperrors.Wrap("INTERNAL_ERROR", "查询岗位列表失败", 500, err)
	}
	return list, total, nil
}
