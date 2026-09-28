package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"github.com/gin-gonic/gin"
)

func TestPatchPropertyAccessAndID(t *testing.T) {
	admin, err := auth.GenerateToken(1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	user, err := auth.GenerateToken(2, "user")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	// A nil service makes any unexpected attempt to access persistence fail the test.
	SetUpRoutes(router, &auth.Handler{}, property.NewPropertyHandler(nil))
	for _, tc := range []struct {
		name, id, token string
		status          int
	}{
		{"missing JWT", "1", "", 401},
		{"invalid JWT", "1", "invalid", 401},
		{"non admin", "1", user, 403},
		{"non numeric ID", "abc", admin, 400},
		{"negative ID", "-1", admin, 400},
		{"zero ID", "0", admin, 400},
		{"overflow ID", "18446744073709551616", admin, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/properties/"+tc.id, strings.NewReader(`{"price":45000000}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tc.status, recorder.Body.String())
			}
		})
	}
}

func TestDeletePropertyAccessAndID(t *testing.T) {
	admin, err := auth.GenerateToken(1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	user, err := auth.GenerateToken(2, "user")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	SetUpRoutes(router, &auth.Handler{}, property.NewPropertyHandler(nil))
	for _, tc := range []struct {
		name, id, token string
		status          int
	}{
		{"missing JWT", "4", "", http.StatusUnauthorized},
		{"invalid JWT", "4", "invalid", http.StatusUnauthorized},
		{"non admin", "4", user, http.StatusForbidden},
		{"non numeric ID", "abc", admin, http.StatusBadRequest},
		{"negative ID", "-1", admin, http.StatusBadRequest},
		{"zero ID", "0", admin, http.StatusBadRequest},
		{"overflow ID", "18446744073709551616", admin, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/properties/"+tc.id, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tc.status, recorder.Body.String())
			}
		})
	}
}
