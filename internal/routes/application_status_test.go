package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"dev/api-task-manager/internal/features/application"
)

func TestApplicationStatusAccessAndValidation(t *testing.T) {
	admin := applicationToken(t, 33, "admin")
	user := applicationToken(t, 11, "user")
	// Nil persistence proves authorization and validation stop before DB access.
	router := applicationRouter(nil)
	for _, tc := range []struct {
		name, id, token, body string
		status                int
	}{
		{"missing JWT", "1", "", `{"status":"approved"}`, 401},
		{"invalid JWT", "1", "invalid", `{"status":"approved"}`, 401},
		{"non admin", "1", user, `{"status":"approved"}`, 403},
		{"non numeric ID", "abc", admin, `{"status":"approved"}`, 400},
		{"negative ID", "-1", admin, `{"status":"approved"}`, 400},
		{"zero ID", "0", admin, `{"status":"approved"}`, 400},
		{"overflow ID", "18446744073709551616", admin, `{"status":"approved"}`, 400},
		{"exceeds PostgreSQL bigint", "9223372036854775808", admin, `{"status":"approved"}`, 400},
		{"missing status", "1", admin, `{}`, 400},
		{"null status", "1", admin, `{"status":null}`, 400},
		{"pending target", "1", admin, `{"status":"pending"}`, 400},
		{"unknown status", "1", admin, `{"status":"cancelled"}`, 400},
		{"case sensitive status", "1", admin, `{"status":"Approved"}`, 400},
		{"wrong status type", "1", admin, `{"status":1}`, 400},
		{"malformed JSON", "1", admin, `{"status":`, 400},
		{"empty body", "1", admin, "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := applicationRequest(router, http.MethodPatch, "/api/v1/admin/applications/"+tc.id+"/status", tc.token, tc.body)
			requireApplicationStatus(t, response, tc.status)
		})
	}
}

func TestApplicationStatusPostgresTransitions(t *testing.T) {
	db := applicationDatabase(t, true)
	router := applicationRouter(application.NewApplicationService(application.NewApplicationRepository(db)))
	admin := applicationToken(t, 33, "admin")
	userID := uint(100)
	for _, initial := range []string{"pending", "approved", "rejected"} {
		for _, target := range []string{"approved", "rejected"} {
			t.Run(initial+" to "+target, func(t *testing.T) {
				userID++
				oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				app := application.Application{UserID: userID, PropertyID: 7, Status: initial, CreatedAt: oldTime, UpdatedAt: oldTime}
				if err := db.Create(&app).Error; err != nil {
					t.Fatal(err)
				}
				path := fmt.Sprintf("/api/v1/admin/applications/%d/status", app.ID)
				response := applicationRequest(router, http.MethodPatch, path, admin, fmt.Sprintf(`{"status":%q}`, target))
				if initial == "pending" {
					requireApplicationStatus(t, response, http.StatusOK)
					var updated application.Application
					if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
						t.Fatal(err)
					}
					if updated.ID != app.ID || updated.UserID != app.UserID || updated.PropertyID != app.PropertyID || updated.Status != target || !updated.CreatedAt.Equal(oldTime) || !updated.UpdatedAt.After(oldTime) {
						t.Fatalf("unexpected updated application: %+v", updated)
					}
				} else {
					requireApplicationStatus(t, response, http.StatusConflict)
				}
				var stored application.Application
				if err := db.First(&stored, app.ID).Error; err != nil {
					t.Fatal(err)
				}
				want := initial
				if initial == "pending" {
					want = target
				}
				if stored.Status != want {
					t.Fatalf("stored status = %q, want %q", stored.Status, want)
				}
			})
		}
	}
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, "/api/v1/admin/applications/999999/status", admin, `{"status":"approved"}`), http.StatusNotFound)

	// An unexpected database failure must not be mistaken for 404 or 409.
	if err := db.Migrator().DropTable(&application.Application{}); err != nil {
		t.Fatal(err)
	}
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, "/api/v1/admin/applications/1/status", admin, `{"status":"approved"}`), http.StatusInternalServerError)
}

func TestApplicationStatusPostgresConcurrentUpdates(t *testing.T) {
	db := applicationDatabase(t, true)
	router := applicationRouter(application.NewApplicationService(application.NewApplicationRepository(db)))
	admin := applicationToken(t, 33, "admin")
	app := application.Application{UserID: 11, PropertyID: 7, Status: "pending"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/v1/admin/applications/%d/status", app.ID)
	type result struct {
		code   int
		target string
	}
	const attempts = 12
	start := make(chan struct{})
	results := make(chan result, attempts)
	var pending sync.WaitGroup
	for i := 0; i < attempts; i++ {
		target := []string{"approved", "rejected"}[i%2]
		pending.Add(1)
		go func(target string) {
			defer pending.Done()
			<-start
			response := applicationRequest(router, http.MethodPatch, path, admin, fmt.Sprintf(`{"status":%q}`, target))
			results <- result{response.Code, target}
		}(target)
	}
	close(start)
	pending.Wait()
	close(results)
	successes, conflicts := 0, 0
	var winningStatus string
	for result := range results {
		switch result.code {
		case http.StatusOK:
			successes++
			winningStatus = result.target
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("unexpected status: %d", result.code)
		}
	}
	if successes != 1 || conflicts != attempts-1 {
		t.Fatalf("successes = %d, conflicts = %d; want 1 and %d", successes, conflicts, attempts-1)
	}
	var stored application.Application
	if err := db.First(&stored, app.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != winningStatus {
		t.Fatalf("stored status = %q, want winner %q", stored.Status, winningStatus)
	}
}
