package services

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go_chat_backend/config"
	"go_chat_backend/pkg/logging"
	"time"
)

type StorageService struct {
	client           *minio.Client
	config           *minio.Options
	bucket           string
	storageType      string
	fileKeyGenerator *FileKeyGenerator
}
type PresignedUploadResult struct {
	UploadURL string            `json:"upload_url"`
	FileKey   string            `json:"file_key"`
	Fields    map[string]string `json:"fields,omitempty"`
	Expires   time.Time         `json:"expires"`
	Provider  string            `json:"provider"` // "minio" or "s3"
}

func InitStorageService(cfg *config.Config) (*StorageService, error) {
	var minioClient *minio.Client
	var err error

	switch cfg.StorageType {
	case "minio":
		minioClient, err = createMinIOClient(cfg)
	case "s3":
		minioClient, err = createS3Client(cfg)
	default:
		logging.Logger.Error("fail InitStorageService", err)
		return nil, err
	}
	if err != nil {
		logging.Logger.Error("fail InitStorageService", err)
		return nil, err
	}
	keyGenerator := NewFileKeyGenerator(StrategyDateBased, "pdfs")
	ss := &StorageService{
		client:           minioClient,
		config:           &minio.Options{Region: cfg.BucketRegion},
		bucket:           cfg.BucketName,
		storageType:      cfg.StorageType,
		fileKeyGenerator: keyGenerator,
	}
	if err := ss.ensureBucketExists(); err != nil {
		logging.Logger.Error("fail InitStorageService", err)
		return nil, err
	}
	logging.Logger.Info("Storage service initialized",
		"type", cfg.StorageType,
		"bucket", cfg.BucketName,
		"region", cfg.BucketRegion,
	)

	return ss, nil
}

func (ss *StorageService) ensureBucketExists() error {
	ctx := context.Background()
	exists, err := ss.client.BucketExists(ctx, ss.bucket)
	if err != nil {
		logging.Logger.Error("fail ensureBucketExists", err)
		return err
	}
	if exists {
		logging.Logger.Info("Bucket already exists")
		return nil
	}
	err = ss.client.MakeBucket(ctx, ss.bucket, minio.MakeBucketOptions{
		Region: ss.config.Region,
	})
	if err != nil {
		if ss.storageType == "s3" {
			logging.Logger.Warn("Could not create S3 bucket (might exist or no permission)",
				"bucket", ss.bucket, "error", err)
			return nil
		}
		logging.Logger.Error("fail ensureBucketExists", err)
		return err
	}
	logging.Logger.Info("Bucket created successfully")
	return nil
}

func (ss *StorageService) GeneratePresignedURL(filename string) (*PresignedUploadResult, error) {
	fileKey := ss.fileKeyGenerator.GenerateFileKey(filename, "")
	uploadURL, err := ss.client.PresignedPutObject(
		context.Background(),
		ss.bucket,
		fileKey,
		15*time.Minute)
	if err != nil {
		logging.Logger.Error("fail GeneratePresignedURL", err)
		return nil, err
	}
	return &PresignedUploadResult{
		UploadURL: uploadURL.String(),
		FileKey:   fileKey,
		Expires:   time.Now().Add(15 * time.Minute),
		Provider:  ss.storageType,
	}, nil
}

func (ss *StorageService) GeneratePresignedPostUpload(filename string, maxFileSize int64) (*PresignedUploadResult, error) {
	fileKey := ss.fileKeyGenerator.GenerateFileKey(filename, "")

	policy := minio.NewPostPolicy()
	err := policy.SetBucket(ss.bucket)
	if err != nil {
		return nil, err
	}
	err = policy.SetKey(fileKey)
	if err != nil {
		return nil, err
	}
	err = policy.SetExpires(time.Now().Add(15 * time.Minute))
	if err != nil {
		return nil, err
	}

	if maxFileSize > 0 {
		err := policy.SetContentLengthRange(1, maxFileSize)
		if err != nil {
			return nil, err
		}
	}
	err = policy.SetContentType("application/pdf")
	if err != nil {
		return nil, err
	}

	postURL, formData, err := ss.client.PresignedPostPolicy(context.Background(), policy)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned POST: %w", err)
	}

	return &PresignedUploadResult{
		UploadURL: postURL.String(),
		FileKey:   fileKey,
		Fields:    formData,
		Expires:   time.Now().Add(15 * time.Minute),
		Provider:  ss.storageType,
	}, nil
}

func createMinIOClient(cfg *config.Config) (*minio.Client, error) {
	return minio.New(cfg.BucketEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.BucketAccessID, cfg.BucketAccessKey, ""),
		Secure: cfg.UseSSL,
	})
}
func createS3Client(cfg *config.Config) (*minio.Client, error) {
	return minio.New("s3.amazonaws.com", &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.BucketAccessID, cfg.BucketAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.BucketRegion,
	})
}
