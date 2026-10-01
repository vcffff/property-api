package auth

import (
	"context"
	"dev/api-task-manager/internal/features/user"
	"dev/api-task-manager/internal/session"
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ServiceAuth struct {
	db             *gorm.DB
	sessionService *session.SessionService
}

func NewServiceAuth(db *gorm.DB, sessionService *session.SessionService) *ServiceAuth {
	return &ServiceAuth{
		db:             db,
		sessionService: sessionService,
	}
}

func (thisServiceAuth *ServiceAuth) Register(req RegisterRequest) (*user.User, error) {
	var existingUser user.User
	result := thisServiceAuth.db.Where("email = ?", req.Email).First(&existingUser)

	if result.Error == nil {
		return nil, errors.New("user already exists")
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	newUser := user.User{
		Email:    req.Email,
		Password: string(passwordHash),
		Role:     "user",
	}

	if err := thisServiceAuth.db.Create(&newUser).Error; err != nil {
		return nil, err
	}
	return &newUser, nil

}

func (s *ServiceAuth) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {

	var user user.User

	result := s.db.
		Where("email = ?", req.Email).
		First(&user)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.New("invalid email or password")
	}

	if result.Error != nil {
		return nil, result.Error
	}

	err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(req.Password),
	)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	accessToken, err := GenerateToken(
		user.ID,
		user.Role,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.sessionService.Create(
		ctx,
		user.ID,
		user.Role,
	)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
		Role:         user.Role,
	}, nil
}

func (s *ServiceAuth) GetAllUsers() ([]user.User, error) {
	var users []user.User

	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *ServiceAuth) GetUserByID(userID uint) (*user.User, error) {
	var user user.User

	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *ServiceAuth) RefreshToken(ctx context.Context, refreshToken string) (*RefreshTokenResponse, error) {
	currentSession, err := s.sessionService.Consume(ctx, refreshToken)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}
	newRefreshToken, err := s.sessionService.Create(
		ctx,
		currentSession.UserID,
		currentSession.Role,
	)
	if err != nil {
		return nil, err
	}

	newAccessToken, err := GenerateToken(
		currentSession.UserID,
		currentSession.Role,
	)

	if err != nil {
		return nil, err
	}

	return &RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}, nil
}

func (s *ServiceAuth) Logout(ctx context.Context, refreshToken string) error {
	return s.sessionService.Delete(ctx, refreshToken)
}
