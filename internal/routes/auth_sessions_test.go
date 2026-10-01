package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func sessionTestRouter(t *testing.T) (*gin.Engine, *session.SessionService) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Skipf("Redis is unavailable: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	sessions := session.NewService(client)
	router := gin.New()
	SetUpRoutes(router, auth.NewHandler(auth.NewServiceAuth(nil, sessions)), property.NewPropertyHandler(nil))
	return router, sessions
}

func sessionRequest(router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"refresh_token":"`+token+`"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestRefreshAndLogoutLifecycle(t *testing.T) {
	router, sessions := sessionTestRouter(t)
	ctx := context.Background()
	first, err := sessions.Create(ctx, 42, "user")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Delete(ctx, first) })
	response := sessionRequest(router, "/api/v1/auth/refresh", first)
	if response.Code != http.StatusOK {
		t.Fatalf("refresh status = %d: %s", response.Code, response.Body.String())
	}
	var rotated auth.RefreshTokenResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken == "" || rotated.RefreshToken == first || rotated.AccessToken == "" {
		t.Fatalf("invalid rotated token response: %+v", rotated)
	}
	t.Cleanup(func() { sessions.Delete(ctx, rotated.RefreshToken) })
	if response := sessionRequest(router, "/api/v1/auth/refresh", first); response.Code != http.StatusUnauthorized {
		t.Fatalf("reused token status = %d, want 401", response.Code)
	}
	if response := sessionRequest(router, "/api/v1/auth/logout", rotated.RefreshToken); response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204: %s", response.Code, response.Body.String())
	}
	if response := sessionRequest(router, "/api/v1/auth/refresh", rotated.RefreshToken); response.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out token status = %d, want 401", response.Code)
	}
}

func TestConcurrentRefreshIsSingleUse(t *testing.T) {
	router, sessions := sessionTestRouter(t)
	ctx := context.Background()
	token, err := sessions.Create(ctx, 42, "user")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sessions.Delete(ctx, token) })

	const attempts = 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes atomic.Int32
	responses := make(chan *httptest.ResponseRecorder, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			response := sessionRequest(router, "/api/v1/auth/refresh", token)
			if response.Code == http.StatusOK {
				successes.Add(1)
			}
			responses <- response
		}()
	}
	close(start)
	wg.Wait()
	close(responses)
	for response := range responses {
		if response.Code == http.StatusOK {
			var rotated auth.RefreshTokenResponse
			if err := json.Unmarshal(response.Body.Bytes(), &rotated); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { sessions.Delete(ctx, rotated.RefreshToken) })
		} else if response.Code != http.StatusUnauthorized {
			t.Fatalf("unexpected refresh status %d: %s", response.Code, response.Body.String())
		}
	}
	if successes.Load() != 1 {
		t.Fatalf("expected exactly 1 successful refresh, got %d", successes.Load())
	}
}
