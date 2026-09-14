package job

import "time"

// Category 岗位分类
type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Job 岗位
type Job struct {
	ID           string    `json:"id"`
	CategoryID   string    `json:"category_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	Requirements []string  `json:"requirements"`
	Skills       []string  `json:"skills"`
	Source       string    `json:"source,omitempty"`
	SourceURL    string    `json:"source_url,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateJobRequest 创建岗位请求
type CreateJobRequest struct {
	CategoryID   string   `json:"category_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Requirements []string `json:"requirements"`
	Skills       []string `json:"skills"`
	Source       string   `json:"source"`
	SourceURL    string   `json:"source_url"`
}

// ListJobsRequest 岗位列表查询
type ListJobsRequest struct {
	CategoryID string `json:"category_id"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}
