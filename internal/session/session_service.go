package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

type SessionService struct {
	redisClient *redis.Client
}

func NewService(redisClient *redis.Client) *SessionService {
	return &SessionService{
		redisClient: redisClient,
	}
}
func (s *SessionService) Create(ctx context.Context,
	userID uint,
	role string) (string, error) {
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	tokenHash := HashRefreshToken(refreshToken)
	session := Session{
		UserID: userID,
		Role:   role,
	}
	data, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	key := "session:" + tokenHash
	err = s.redisClient.Set(ctx, key, data, 7*24*time.Hour).Err()
	if err != nil {
		return "", err
	}
	return refreshToken, nil
}

func (s *SessionService) Get(ctx context.Context, refreshToken string) (*Session, error) {
	tokenHash := HashRefreshToken(refreshToken)
	key := "session:" + tokenHash
	data, err := s.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// Consume returns and deletes a refresh session in one atomic Redis command.
func (s *SessionService) Consume(ctx context.Context, refreshToken string) (*Session, error) {
	tokenHash := HashRefreshToken(refreshToken)
	key := "session:" + tokenHash
	data, err := s.redisClient.GetDel(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *SessionService) Delete(
	ctx context.Context,
	refreshToken string,
) error {

	tokenHash := HashRefreshToken(refreshToken)

	key := "session:" + tokenHash

	return s.redisClient.Del(ctx, key).Err()
}
