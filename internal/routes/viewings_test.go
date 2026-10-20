package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"dev/api-task-manager/internal/features/application"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/viewing"
	"github.com/gin-gonic/gin"
)

func TestMyViewingsRequiresJWT(t *testing.T) {
	router := applicationRouter(nil)
	for _, token := range []string{"", "invalid"} {
		requireApplicationStatus(t, applicationRequest(router, http.MethodGet, "/api/v1/viewings/me", token, ""), http.StatusUnauthorized)
	}
}

func TestMyViewingsPostgresOwnership(t *testing.T) {
	db := applicationDatabase(t, false)
	if err := db.AutoMigrate(&viewing.Viewing{}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	SetUpRoutes(router, &auth.Handler{}, property.NewPropertyHandler(nil), application.NewApplicationHandler(nil),
		viewing.NewViewingHandler(viewing.NewViewingService(viewing.NewViewingRepository(db))))
	rows := []viewing.Viewing{
		{UserID: 11, PropertyID: 2, StartsAt: time.Now().UTC(), Status: viewing.ViewingStatusPending},
		{UserID: 22, PropertyID: 2, StartsAt: time.Now().UTC(), Status: viewing.ViewingStatusConfirmed},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, role string
		userID     uint
		wantCount  int
	}{
		{"first user", "user", 11, 1},
		{"second user", "user", 22, 1},
		{"admin only sees own", "admin", 33, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Neither query parameters nor a body may override JWT ownership.
			response := applicationRequest(router, http.MethodGet, "/api/v1/viewings/me?user_id=22", applicationToken(t, tc.userID, tc.role), `{"user_id":22}`)
			requireApplicationStatus(t, response, http.StatusOK)
			var items []viewing.Viewing
			if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
				t.Fatal(err)
			}
			if items == nil || len(items) != tc.wantCount {
				t.Fatalf("expected JSON array of length %d, got %s", tc.wantCount, response.Body.String())
			}
			for _, item := range items {
				if item.UserID != tc.userID {
					t.Fatalf("another user's viewing leaked: %+v", item)
				}
			}
		})
	}
}

func TestViewingBookingAccessAndValidation(t *testing.T) {
	router := applicationRouter(nil)
	user := applicationToken(t, 11, "user")
	admin := applicationToken(t, 33, "admin")
	for _, tc := range []struct {
		name, token, body, message string
		status                     int
	}{
		{"missing JWT", "", `{}`, "", 401},
		{"invalid JWT", "invalid", `{}`, "", 401},
		{"missing fields", user, `{}`, "", 400},
		{"zero property", user, `{"property_id":0,"starts_at":"2030-10-24T05:00:00Z"}`, "", 400},
		{"negative property", user, `{"property_id":-1,"starts_at":"2030-10-24T05:00:00Z"}`, "", 400},
		{"oversize property", user, `{"property_id":9223372036854775808,"starts_at":"2030-10-24T05:00:00Z"}`, "invalid property ID", 400},
		{"missing timezone", user, `{"property_id":1,"starts_at":"2030-10-24T10:00:00"}`, "", 400},
		{"malformed JSON", user, `{"property_id":`, "", 400},
		{"user reaches service", user, `{"property_id":1,"starts_at":"2000-01-01T15:00:00+05:00"}`, "cannot book a viewing in the past", 400},
		{"admin reaches service", admin, `{"property_id":1,"starts_at":"2000-01-01T15:00:00+05:00"}`, "cannot book a viewing in the past", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := applicationRequest(router, http.MethodPost, "/api/v1/viewings", tc.token, tc.body)
			requireApplicationStatus(t, response, tc.status)
			if tc.message != "" && !strings.Contains(response.Body.String(), tc.message) {
				t.Fatalf("expected service validation error %q, got %s", tc.message, response.Body.String())
			}
		})
	}
}

func TestAvailableSlotsAccessAndValidation(t *testing.T) {
	router := applicationRouter(nil)
	user := applicationToken(t, 11, "user")
	admin := applicationToken(t, 33, "admin")
	for _, tc := range []struct {
		name, id, date, token, message string
		status                         int
	}{
		{"missing JWT", "1", "2026-10-24", "", "", 401},
		{"invalid JWT", "1", "2026-10-24", "invalid", "", 401},
		{"invalid ID", "abc", "2026-10-24", user, "invalid property ID", 400},
		{"zero ID", "0", "2026-10-24", user, "invalid property ID", 400},
		{"negative ID", "-1", "2026-10-24", user, "invalid property ID", 400},
		{"overflow ID", "9223372036854775808", "2026-10-24", user, "invalid property ID", 400},
		{"missing date", "1", "", user, "invalid date", 400},
		{"user reaches service", "1", "2026-02-30", user, "invalid date", 400},
		{"admin reaches service", "1", "invalid", admin, "invalid date", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/api/v1/properties/" + tc.id + "/available-slots?date=" + tc.date
			response := applicationRequest(router, http.MethodGet, path, tc.token, "")
			requireApplicationStatus(t, response, tc.status)
			if tc.message != "" && !strings.Contains(response.Body.String(), tc.message) {
				t.Fatalf("expected error %q, got %s", tc.message, response.Body.String())
			}
		})
	}
}
