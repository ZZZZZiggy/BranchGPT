package models

import "time"

type Document struct {
	DocID     string `gorm:"primaryKey"`
	Title     string
	Status    string
	CreatedAt time.Time
	Root      string
}
