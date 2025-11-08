package database

import (
	"fmt"
	"go_chat_backend/config"
	"go_chat_backend/models"
	"go_chat_backend/pkg/logging"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"time"
)

type DB struct {
	database *gorm.DB
}

func InitPostgres(cfg *config.Config) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=prefer TimeZone=UTC",
		cfg.Host,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.Port,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logging.Logger.Error("failed to connect to database", "error", err)
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		logging.Logger.Error("failed to connect to database", "error", err)
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB := &DB{database: db}
	fmt.Println("Connected to Postgres")

	return DB, nil
}
func (db *DB) AutoMigrate() error {
	if err := db.database.AutoMigrate(&models.ChatNode{}); err != nil {
		logging.Logger.Error("auto migration failed", "error", err)
		return err
	}
	if err := db.database.AutoMigrate(&models.DocumentMeta{}); err != nil {
		logging.Logger.Error("auto migration failed", "error", err)
		return err
	}
	if err := db.database.AutoMigrate(&models.Chunk{}); err != nil {
		logging.Logger.Error("auto migration failed", "error", err)
		return err
	}

	return nil
}
func (db *DB) Close() error {
	sqlDB, err := db.database.DB()
	if err != nil {
		logging.Logger.Error("failed to connect to database", "error", err)
		return err
	}
	return sqlDB.Close()
}
func (db *DB) GetDatabase() *gorm.DB {
	return db.database
}
func (db *DB) Ping() error {
	sqlDB, err := db.database.DB()
	if err != nil {
		logging.Logger.Error("failed to connect to database", "error", err)
		return err
	}
	return sqlDB.Ping()
}
