package viewing

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPastViewing       = errors.New("viewing must be in the future")
	ErrInvalidSlot       = errors.New("invalid viewing time slot")
	ErrPropertyGone      = errors.New("property not found")
	ErrInvalidPropertyID = errors.New("invalid property ID")
	ErrInvalidRange      = errors.New("invalid viewing time range")
	ErrUnauthorized      = errors.New("unauthorized")
)

type ViewingService struct {
	repository *ViewingRepository
}

func NewViewingService(repo *ViewingRepository) *ViewingService {
	return &ViewingService{
		repository: repo,
	}
}

func (s *ViewingService) Book(
	ctx context.Context,
	userID uint,
	req CreateViewingRequest,
) (*Viewing, error) {
	if userID == 0 {
		return nil, ErrUnauthorized
	}
	if req.PropertyID == 0 || uint64(req.PropertyID) > 1<<63-1 {
		return nil, ErrInvalidPropertyID
	}

	startsAt := req.StartsAt.UTC()

	// Нельзя бронировать прошлое
	if !startsAt.After(time.Now().UTC()) {
		return nil, ErrPastViewing
	}

	// Проверяем локальное время Астаны
	location := time.FixedZone("Astana", 5*60*60)
	localTime := startsAt.In(location)

	hour := localTime.Hour()
	minute := localTime.Minute()

	// Слоты с 10:00 до 18:30 включительно
	if hour < 10 || hour >= 19 {
		return nil, ErrInvalidSlot
	}

	if minute != 0 && minute != 30 {
		return nil, ErrInvalidSlot
	}

	if localTime.Second() != 0 || localTime.Nanosecond() != 0 {
		return nil, ErrInvalidSlot
	}

	exists, err := s.repository.PropertyExists(ctx, req.PropertyID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrPropertyGone
	}

	viewing := &Viewing{
		UserID:     userID,
		PropertyID: req.PropertyID,
		StartsAt:   startsAt,
		Status:     ViewingStatusPending,
	}

	if err := s.repository.Create(ctx, viewing); err != nil {
		return nil, err
	}

	return viewing, nil
}

func (s *ViewingService) GetAvailableSlots(
	ctx context.Context,
	propertyID uint,
	date string,
) (*AvailableSlotsResponse, error) {
	if propertyID == 0 || uint64(propertyID) > 1<<63-1 {
		return nil, ErrInvalidPropertyID
	}
	loc := time.FixedZone("Astana", 5*60*60)
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil || day.Year() < 1 {
		return nil, ErrInvalidSlot
	}

	exists, err := s.repository.PropertyExists(ctx, propertyID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrPropertyGone
	}

	booked, err := s.repository.GetBookedViewings(ctx, propertyID, day.UTC(), day.AddDate(0, 0, 1).UTC())
	if err != nil {
		return nil, err
	}

	return &AvailableSlotsResponse{
		PropertyID: propertyID,
		Date:       date,
		Slots:      buildAvailableSlots(day, booked, time.Now().UTC()),
	}, nil
}

func (s *ViewingService) GetMine(ctx context.Context, userID uint) ([]Viewing, error) {
	if userID == 0 {
		return nil, ErrUnauthorized
	}
	return s.repository.GetByUserID(ctx, userID)
}

func (s *ViewingService) GetAll(ctx context.Context) ([]Viewing, error) {
	return s.repository.GetAll(ctx)
}

func (s *ViewingService) UpdateStatus(ctx context.Context, id uint, status ViewingStatus) (*Viewing, error) {
	if id == 0 {
		return nil, ErrInvalidViewingID
	}
	if !status.IsValid() {
		return nil, ErrInvalidStatus
	}
	return s.repository.UpdateStatus(ctx, id, status)
}

func buildAvailableSlots(day time.Time, booked []time.Time, now time.Time) []AvailableSlot {
	occupied := make(map[int64]bool, len(booked))
	for _, startsAt := range booked {
		occupied[startsAt.UTC().Unix()] = true
	}

	slots := make([]AvailableSlot, 0, 18)
	for hour := 10; hour < 19; hour++ {
		for _, minute := range []int{0, 30} {
			start := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
			slots = append(slots, AvailableSlot{
				StartsAt:  start.UTC(),
				Available: start.After(now) && !occupied[start.UTC().Unix()],
			})
		}
	}
	return slots
}
