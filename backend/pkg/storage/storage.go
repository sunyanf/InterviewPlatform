package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo 文件信息
type FileInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
}

// Storage 对象存储接口
type Storage interface {
	// Upload 上传文件
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	// Download 下载文件
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	// GetSignedURL 获取临时访问 URL
	GetSignedURL(ctx context.Context, key string, expire time.Duration) (string, error)
	// Exists 判断对象是否存在
	Exists(ctx context.Context, key string) (bool, error)
	// Delete 删除文件
	Delete(ctx context.Context, key string) error
}
