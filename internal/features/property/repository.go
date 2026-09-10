package property

import (
	"gorm.io/gorm"
)

type PropertyRepository struct {
	DB *gorm.DB
}

func NewPropertyRepository(db *gorm.DB) *PropertyRepository {
	return &PropertyRepository{
		DB: db,
	}
}

func (r *PropertyRepository) Create(property *Property) error {
	return r.DB.Create(property).Error
}
