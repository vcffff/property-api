package property

import (
	"context"
	"strconv"
	"time"

	"dev/api-task-manager/internal/platform/kafka"
)

type PropertyService struct {
	Repository *PropertyRepository
	Producer   *kafka.Producer
}

func NewPropertyService(repo *PropertyRepository, producer *kafka.Producer) *PropertyService {
	return &PropertyService{
		Repository: repo,
		Producer:   producer,
	}
}
func (s *PropertyService) CreateProperty(ctx context.Context, property *Property) error {
	if err := s.Repository.Create(property); err != nil {
		return err
	}
	event := kafka.PropertyCreatedEvent{
		EventID:    strconv.FormatUint(uint64(property.ID), 10),
		PropertyID: strconv.FormatUint(uint64(property.ID), 10),
		Title:      property.Title,
		CreatedAt:  time.Now().Unix(),
	}
	if err := s.Producer.PublishPropertyCreated(ctx, event); err != nil {
		return err
	}
	return nil

}

func (s *PropertyService) GetAllProperties() ([]Property, error) {
	return s.Repository.GetAll()
}

func (s *PropertyService) GetPropertyByID(id uint) (*Property, error) {
	return s.Repository.GetByID(id)
}

func (s *PropertyService) GetAll() ([]Property, error) {
	return s.Repository.GetAll()
}

func (s *PropertyService) UpdateProperty(id uint, property *UpdatePropertyRequest) (*Property, error) {
	updates := make(map[string]interface{})
	if property.Title != nil {
		updates["title"] = *property.Title
	}
	if property.Address != nil {
		updates["address"] = *property.Address
	}
	if property.Price != nil {
		updates["price"] = *property.Price
	}
	return s.Repository.Update(id, updates)
}

func (s *PropertyService) DeleteProperty(id uint) error {
	return s.Repository.Delete(id)
}
