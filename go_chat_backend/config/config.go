package config

import (
	"os"
	"time"
)

type Config struct {
	// S3/MinIO
	BucketEndpoint  string
	BucketAccessID  string
	BucketAccessKey string
	BucketName      string
	BucketRegion    string
	UseSSL          bool   // MinIO: false, S3: true
	StorageType     string //"minio" or "s3"

	// Redis
	RedisURL      string
	RedisPassword string

	// others
	UploadTimeout time.Duration
	MaxFileSize   int64
}

func LoadConfig() *Config {
	return &Config{
		BucketEndpoint:  os.Getenv("BUCKET_ENDPOINT"),
		BucketAccessID:  os.Getenv("BUCKET_ACCESS_ID"),
		BucketAccessKey: os.Getenv("BUCKET_ACCESS_KEY"),
		BucketName:      os.Getenv("BUCKET_NAME"),
		BucketRegion:    os.Getenv("BUCKET_REGION"),
		RedisURL:        os.Getenv("REDIS_URL"),
		UseSSL:          os.Getenv("BUCKET_USE_SSL") == "true",
		StorageType:     os.Getenv("STORAGE_TYPE"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		UploadTimeout:   15 * time.Minute, // url lifetime
		MaxFileSize:     50 * 1024 * 1024, // 50MB
	}
}
