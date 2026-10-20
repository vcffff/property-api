package viewing

import "time"

type Viewing struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	UserID     uint `json:"user_id" gorm:"not null;index"`
	PropertyID uint `json:"property_id" gorm:"not null;index"`

	StartsAt time.Time `json:"starts_at" gorm:"not null"`

	Status ViewingStatus `json:"status" gorm:"type:text;not null;default:'pending'"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
