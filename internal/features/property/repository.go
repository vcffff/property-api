package property

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *PropertyRepository) Update(id uint, updates map[string]interface{}) (*Property, error) {
	if len(updates) == 0 {
		return r.GetByID(id)
	}
	property := Property{ID: id}
	result := r.db.Model(&property).Clauses(clause.Returning{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &property, nil
}

func (r *PropertyRepository) Delete(id uint) error {
	result := r.db.Delete(&Property{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
