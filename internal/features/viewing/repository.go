package viewing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ViewingRepository struct {
	db *gorm.DB
}

func NewViewingRepository(db *gorm.DB) *ViewingRepository {
	return &ViewingRepository{db: db}
}

var (
	ErrSlotTaken         = errors.New("viewing slot already booked")
	ErrInvalidStatus     = errors.New("invalid viewing status")
	ErrInvalidTransition = errors.New("invalid viewing status transition")
	ErrInvalidViewingID  = errors.New("invalid viewing ID")
)

func (r *ViewingRepository) Create(
	ctx context.Context,
	viewing *Viewing,
) error {
	err := r.db.WithContext(ctx).Create(viewing).Error

	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == activeSlotIndex {
		return ErrSlotTaken
	}
	return err
}

func (r *ViewingRepository) PropertyExists(
	ctx context.Context,
	propertyID uint,
) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Table("properties").
		Where("id = ?", propertyID).
		Count(&count).Error

	return count > 0, err
}

func (r *ViewingRepository) GetBookedViewings(
	ctx context.Context,
	propertyID uint,
	from, to time.Time,
) ([]time.Time, error) {
	if propertyID == 0 {
		return nil, ErrInvalidPropertyID
	}
	if !from.Before(to) {
		return nil, ErrInvalidRange
	}
	var bookedViewings []time.Time

	err := r.db.WithContext(ctx).
		Model(&Viewing{}).
		Where("property_id = ? AND starts_at >= ? AND starts_at < ? AND status IN ?", propertyID, from, to, []ViewingStatus{ViewingStatusPending, ViewingStatusConfirmed}).
		Pluck("starts_at", &bookedViewings).Error
	if err != nil {
		return nil, err
	}
	return bookedViewings, nil
}

func (r *ViewingRepository) GetByUserID(ctx context.Context, userID uint) ([]Viewing, error) {
	if userID == 0 {
		return nil, ErrUnauthorized
	}
	viewings := make([]Viewing, 0)
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("starts_at DESC").Order("id DESC").
		Find(&viewings).Error
	return viewings, err
}

func (r *ViewingRepository) GetAll(ctx context.Context) ([]Viewing, error) {
	viewings := make([]Viewing, 0)
	err := r.db.WithContext(ctx).Order("starts_at DESC").Order("id DESC").Find(&viewings).Error
	return viewings, err
}

func (r *ViewingRepository) UpdateStatus(ctx context.Context, id uint, target ViewingStatus) (*Viewing, error) {
	if id == 0 {
		return nil, ErrInvalidViewingID
	}
	if !target.IsValid() {
		return nil, ErrInvalidStatus
	}
	var updated Viewing
	result := r.db.WithContext(ctx).Model(&updated).Clauses(clause.Returning{}).
		Where("id = ? AND status IN ?", id, transitionSources(target)).
		Updates(map[string]interface{}{"status": target})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return &updated, nil
	}
	// No read-before-write: only the conditional UPDATE decides the transition.
	var count int64
	if err := r.db.WithContext(ctx).Model(&Viewing{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return nil, ErrInvalidTransition
}
