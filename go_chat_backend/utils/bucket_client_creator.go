package utils

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go_chat_backend/config"
)

func CreateMinIOClient(cfg *config.Config) (*minio.Client, error) {
	return minio.New(cfg.BucketEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.BucketAccessID, cfg.BucketAccessKey, ""),
		Secure: cfg.UseSSL,
	})
}
func CreateS3Client(cfg *config.Config) (*minio.Client, error) {
	return minio.New("s3.amazonaws.com", &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.BucketAccessID, cfg.BucketAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.BucketRegion,
	})
}
