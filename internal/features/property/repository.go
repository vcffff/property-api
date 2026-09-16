package property

import (
	"gorm.io/gorm"
)

type PropertyRepository struct {
	db *gorm.DB
}

func NewPropertyRepository(db *gorm.DB) *PropertyRepository {
	return &PropertyRepository{
		db: db,
	}
}

func (r *PropertyRepository) Create(property *Property) error {
	return r.db.Create(property).Error
}



func (r *PropertyRepository) GetAll() ([]Property, error) {
	var properties []Property

	err := r.db.Find(&properties).Error

	return properties, err
}


func (r *PropertyRepository) GetByID(id uint) (*Property, error) {
	var property Property

	err := r.db.First(&property, id).Error
	if err != nil {
		return nil, err
	}

	return &property, nil
}

func (r *PropertyRepository) Update(property *Property) error {
	return r.db.Save(property).Error
}
