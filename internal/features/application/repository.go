package application

import (
	"context"
	"errors"

	"dev/api-task-manager/internal/features/property"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPropertyNotFound  = errors.New("property not found")
	ErrAlreadyApplied    = errors.New("active application already exists")
	ErrInvalidTransition = errors.New("only pending applications can change status")
)

type ApplicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) *ApplicationRepository {
	return &ApplicationRepository{db: db}
}

func (r *ApplicationRepository) UpdateStatus(ctx context.Context, id uint, status string) (*Application, error) {
	var app Application
	result := r.db.WithContext(ctx).Model(&app).
		Clauses(clause.Returning{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{"status": status})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return &app, nil
	}

	// The conditional update is the concurrency guard. This read only
	// distinguishes a missing application from an invalid transition.
	var count int64
	if err := r.db.WithContext(ctx).Model(&Application{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return nil, ErrInvalidTransition
}

func (r *ApplicationRepository) Create(ctx context.Context, app *Application) error {
	err := r.db.WithContext(ctx).Create(app).Error
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == activeApplicationIndex {
		return ErrAlreadyApplied
	}
	return err
}

func (r *ApplicationRepository) GetByUserID(ctx context.Context, userID uint) ([]Application, error) {
	apps := make([]Application, 0)
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").Order("id DESC").
		Find(&apps).Error
	return apps, err
}

func (r *ApplicationRepository) GetAll(ctx context.Context) ([]Application, error) {
	apps := make([]Application, 0)
	err := r.db.WithContext(ctx).
		Order("created_at DESC").Order("id DESC").
		Find(&apps).Error
	return apps, err
}

func (r *ApplicationRepository) PropertyExists(ctx context.Context, propertyID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&property.Property{}).
		Where("id = ?", propertyID).Count(&count).Error
	return count > 0, err
}

func (r *ApplicationRepository) HasActiveApplication(ctx context.Context, userID, propertyID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Application{}).
		Where("user_id = ? AND property_id = ? AND status IN ?", userID, propertyID, []string{"pending", "approved"}).
		Count(&count).Error
	return count > 0, err
}
