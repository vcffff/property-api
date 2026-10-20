package application

import "time"

type Application struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	UserID     uint      `json:"user_id" gorm:"not null;index"`
	PropertyID uint      `json:"property_id" gorm:"not null;index"`
	Status     string    `json:"status" gorm:"not null;default:'pending'"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}
