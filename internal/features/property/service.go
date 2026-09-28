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
