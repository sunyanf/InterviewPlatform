package storage

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"ai-interview-platform/internal/config"
)

// MinIOStorage MinIO 对象存储实现
type MinIOStorage struct {
	client *minio.Client
	bucket string
}

// NewMinIOStorage 创建 MinIO Storage
func NewMinIOStorage(cfg config.StorageConfig) (*MinIOStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	// 确保 bucket 存在
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return &MinIOStorage{client: client, bucket: cfg.Bucket}, nil
}

// Upload 上传文件
func (m *MinIOStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Download 下载文件
func (m *MinIOStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// GetSignedURL 获取临时访问 URL
func (m *MinIOStorage) GetSignedURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucket, key, expire, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// Delete 删除文件
func (m *MinIOStorage) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}
