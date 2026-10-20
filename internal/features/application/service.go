package application

import (
	"context"
	"errors"
)

var ErrInvalidStatus = errors.New("status must be approved or rejected")

type ApplicationService struct {
	repository *ApplicationRepository
}

func NewApplicationService(repository *ApplicationRepository) *ApplicationService {
	return &ApplicationService{repository: repository}
}

func (s *ApplicationService) UpdateStatus(ctx context.Context, id uint, status string) (*Application, error) {
	if status != "approved" && status != "rejected" {
		return nil, ErrInvalidStatus
	}
	return s.repository.UpdateStatus(ctx, id, status)
}

func (s *ApplicationService) Create(ctx context.Context, userID uint, req CreateApplicationRequest) (*Application, error) {
	exists, err := s.repository.PropertyExists(ctx, req.PropertyID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrPropertyNotFound
	}

	alreadyApplied, err := s.repository.HasActiveApplication(ctx, userID, req.PropertyID)
	if err != nil {
		return nil, err
	}
	if alreadyApplied {
		return nil, ErrAlreadyApplied
	}

	app := &Application{
		UserID:     userID,
		PropertyID: req.PropertyID,
		Status:     "pending",
	}
	if err := s.repository.Create(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *ApplicationService) GetMine(ctx context.Context, userID uint) ([]Application, error) {
	return s.repository.GetByUserID(ctx, userID)
}

func (s *ApplicationService) GetAll(ctx context.Context) ([]Application, error) {
	return s.repository.GetAll(ctx)
}
