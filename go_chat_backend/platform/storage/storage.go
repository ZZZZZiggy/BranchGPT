package storage

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"go_chat_backend/config"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"go_chat_backend/utils"
	"time"
)

type Service struct {
	Client           *minio.Client
	Config           *minio.Options
	Bucket           string
	StorageType      string
	FileKeyGenerator *utils.FileKeyGenerator
}

func InitStorageService(cfg *config.Config) (*Service, error) {
	// bucket
	var minioClient *minio.Client
	var err error

	// local vs s3
	switch cfg.StorageType {
	case "minio":
		minioClient, err = utils.CreateMinIOClient(cfg)
	case "s3":
		minioClient, err = utils.CreateS3Client(cfg)
	default:
		logging.Logger.Error("fail InitStorageService, type error", err)
		return nil, err
	}
	if err != nil {
		logging.Logger.Error("fail InitStorageService", err)
		return nil, err
	}
	// generate callback message
	keyGenerator := utils.NewFileKeyGenerator(utils.StrategyDateBased, "pdfs")
	ss := &Service{
		Client:           minioClient,
		Config:           &minio.Options{Region: cfg.BucketRegion},
		Bucket:           cfg.BucketName,
		StorageType:      cfg.StorageType,
		FileKeyGenerator: keyGenerator,
	}
	if err := ss.EnsureBucketExists(); err != nil {
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
func (ss *Service) EnsureBucketExists() error {
	ctx := context.Background()
	exists, err := ss.Client.BucketExists(ctx, ss.Bucket)
	if err != nil {
		logging.Logger.Error("fail ensureBucketExists", err)
		return err
	}
	if exists {
		logging.Logger.Info("Bucket already exists")
		return nil
	}
	err = ss.Client.MakeBucket(ctx, ss.Bucket, minio.MakeBucketOptions{
		Region: ss.Config.Region,
	})
	if err != nil {
		if ss.StorageType == "s3" {
			logging.Logger.Warn("Could not create S3 bucket (might exist or no permission)",
				"bucket", ss.Bucket, "error", err)
			return nil
		}
		logging.Logger.Error("fail ensureBucketExists", err)
		return err
	}
	logging.Logger.Info("Bucket created successfully")
	return nil
}

func (ss *Service) GeneratePresignedPostUpload(filename string, maxFileSize int64, docID string) (*models.UploadResp, error) {
	fileKey := ss.FileKeyGenerator.GenerateFileKey(filename, "")

	policy := minio.NewPostPolicy()
	err := policy.SetBucket(ss.Bucket)
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

	postURL, formData, err := ss.Client.PresignedPostPolicy(context.Background(), policy)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned POST: %w", err)
	}

	return &models.UploadResp{
		DocId:     docID,
		UploadURL: postURL.String(),
		FileKey:   fileKey,
		Fields:    formData,
		Expires:   time.Now().Add(15 * time.Minute),
		Provider:  ss.StorageType,
	}, nil
}
func (ss *Service) GeneratePresignedGetDownload(fileKey string, expiration time.Time) (string, error) {
	duration := time.Until(expiration)
	if duration <= 0 {
		logging.Logger.Error("fail GeneratePresignedGetDownload, expiration error", expiration)
		return "", fmt.Errorf("expiration error")
	}
	presignedURL, err := ss.Client.PresignedGetObject(
		context.Background(),
		ss.Bucket,
		fileKey,
		duration,
		nil,
	)
	if err != nil {
		logging.Logger.Error("fail GeneratePresignedGetDownload", err)
		return "", err
	}
	return presignedURL.String(), nil
}

func (ss *Service) FileExists(fileKey string) (bool, error) {
	_, err := ss.Client.StatObject(context.Background(), ss.Bucket, fileKey, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
