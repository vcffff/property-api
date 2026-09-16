package auth

import (
	"dev/api-task-manager/internal/features/user"
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ServiceAuth struct {
	db *gorm.DB
}

func NewServiceAuth(db *gorm.DB) *ServiceAuth {
	return &ServiceAuth{
		db: db,
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

func (s *ServiceAuth) Login(req LoginRequest) (string, string, error) {
	var user user.User

	result := s.db.Where("email = ?", req.Email).First(&user)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", "", errors.New("invalid email or password")
	}

	if result.Error != nil {
		return "", "", result.Error
	}

	err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(req.Password),
	)

	if err != nil {
		return "", "", errors.New("invalid email or password")
	}

	token, err := GenerateToken(user.ID, user.Role)
	if err != nil {
		return "", "", err
	}

	return token, user.Role, nil
}

func (s *ServiceAuth) GetUserByID(userID uint) (*user.User, error) {
	var user user.User

	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *ServiceAuth) GetAllUsers() ([]user.User, error) {
	var users []user.User

	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
