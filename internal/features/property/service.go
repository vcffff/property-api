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
