package config

import (
	"os"
	"time"
)

type Config struct {
	HttpPort string
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

	// Postgres
	Host     string
	User     string
	Password string
	DBName   string
	Port     string

	// others
	UploadTimeout time.Duration
	MaxFileSize   int64

	// grpc
	GoGrpcIngestPort  string
	GrpcServerAddr    string
	GrpcEmbeddingAddr string
}

func LoadConfig() *Config {
	return &Config{
		HttpPort:          os.Getenv("PORT"),
		BucketEndpoint:    os.Getenv("BUCKET_ENDPOINT"),
		BucketAccessID:    os.Getenv("BUCKET_ACCESS_ID"),
		BucketAccessKey:   os.Getenv("BUCKET_ACCESS_KEY"),
		BucketName:        os.Getenv("BUCKET_NAME"),
		BucketRegion:      os.Getenv("BUCKET_REGION"),
		RedisURL:          os.Getenv("REDIS_URL"),
		UseSSL:            os.Getenv("BUCKET_USE_SSL") == "true",
		StorageType:       os.Getenv("STORAGE_TYPE"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		UploadTimeout:     15 * time.Minute,
		MaxFileSize:       50 * 1024 * 1024,
		Host:              os.Getenv("PG_HOST"),
		User:              os.Getenv("PG_USER"),
		Password:          os.Getenv("PG_PASSWORD"),
		DBName:            os.Getenv("PG_DB"),
		Port:              os.Getenv("PG_PORT"),
		GoGrpcIngestPort:  os.Getenv("GO_GRPC_INGEST_PORT"),
		GrpcServerAddr:    os.Getenv("GRPC_SERVER_ADDR"),
		GrpcEmbeddingAddr: os.Getenv("GRPC_EMBEDDING_ADDR"),
	}
}
