package user

import "time"

type User struct {
	ID        uint   `gorm:"primaryKey"`
	Email     string `gorm:"uniqueIndex;not null"`
	Password  string `gorm:"not null"`
	Role      string `gorm:"not null;default:user"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
