package property

type PropertyService struct {
	Repository *PropertyRepository
}

func NewPropertyService(repo *PropertyRepository) *PropertyService {
	return &PropertyService{
		Repository: repo,
	}
}
func (s *PropertyService) CreateProperty(property *Property) error {
	return s.Repository.Create(property)
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
	propertyToUpdate, err := s.Repository.GetByID(id)
	if err != nil {
		return nil, err
	}

	propertyToUpdate.Title = property.Title
	propertyToUpdate.Address = property.Address
	propertyToUpdate.Price = property.Price
	err = s.Repository.Update(propertyToUpdate)
	if err != nil {
		return nil, err
	}
	return propertyToUpdate, nil
}
