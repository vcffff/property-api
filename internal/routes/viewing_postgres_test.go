package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"dev/api-task-manager/internal/features/application"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/viewing"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Reuse the existing opt-in PostgreSQL fixture and its isolated search_path.
func viewingPostgresRouter(t *testing.T, withIndex bool) (*gorm.DB, *gin.Engine) {
	t.Helper()
	db := applicationDatabase(t, false)
	if err := db.AutoMigrate(&viewing.Viewing{}); err != nil {
		t.Fatal(err)
	}
	if withIndex {
		if err := viewing.EnsureActiveSlotIndex(db); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	SetUpRoutes(router, &auth.Handler{}, property.NewPropertyHandler(nil), application.NewApplicationHandler(nil),
		viewing.NewViewingHandler(viewing.NewViewingService(viewing.NewViewingRepository(db))))
	return db, router
}

func decodeViewing(t *testing.T, data []byte) viewing.Viewing {
	t.Helper()
	var item viewing.Viewing
	if err := json.Unmarshal(data, &item); err != nil {
		t.Fatal(err)
	}
	return item
}

func TestViewingPostgresTransitions(t *testing.T) {
	db, router := viewingPostgresRouter(t, true)
	admin := applicationToken(t, 33, "admin")
	states := []viewing.ViewingStatus{viewing.ViewingStatusPending, viewing.ViewingStatusConfirmed, viewing.ViewingStatusCancelled, viewing.ViewingStatusCompleted}
	index := 0
	for _, from := range states {
		for _, to := range states {
			t.Run(string(from)+" to "+string(to), func(t *testing.T) {
				index++
				oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				row := viewing.Viewing{UserID: 11, PropertyID: uint(index), StartsAt: oldTime.Add(time.Hour), Status: from, CreatedAt: oldTime, UpdatedAt: oldTime}
				if err := db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
				path := fmt.Sprintf("/api/v1/admin/viewings/%d/status", row.ID)
				response := applicationRequest(router, http.MethodPatch, path, admin, fmt.Sprintf(`{"status":%q}`, to))
				allowed := from == viewing.ViewingStatusPending && (to == viewing.ViewingStatusConfirmed || to == viewing.ViewingStatusCancelled) ||
					from == viewing.ViewingStatusConfirmed && (to == viewing.ViewingStatusCompleted || to == viewing.ViewingStatusCancelled)
				want := from
				if allowed {
					requireApplicationStatus(t, response, 200)
					updated := decodeViewing(t, response.Body.Bytes())
					if updated.ID != row.ID || updated.UserID != row.UserID || updated.PropertyID != row.PropertyID || !updated.StartsAt.Equal(row.StartsAt) || !updated.CreatedAt.Equal(row.CreatedAt) || updated.Status != to || !updated.UpdatedAt.After(oldTime) {
						t.Fatalf("unexpected returned row: %+v", updated)
					}
					want = to
				} else {
					requireApplicationStatus(t, response, 409)
				}
				var stored viewing.Viewing
				if err := db.First(&stored, row.ID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.Status != want {
					t.Fatalf("stored status = %s, want %s", stored.Status, want)
				}
			})
		}
	}
	for _, target := range states {
		requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, "/api/v1/admin/viewings/999999/status", admin, fmt.Sprintf(`{"status":%q}`, target)), 404)
	}
	response := applicationRequest(router, http.MethodGet, "/api/v1/admin/viewings", admin, "")
	requireApplicationStatus(t, response, 200)
	var all []viewing.Viewing
	if err := json.Unmarshal(response.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 16 {
		t.Fatalf("admin list length = %d", len(all))
	}

	if err := db.Migrator().DropTable(&viewing.Viewing{}); err != nil {
		t.Fatal(err)
	}
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, "/api/v1/admin/viewings/1/status", admin, `{"status":"confirmed"}`), 500)
	requireApplicationStatus(t, applicationRequest(router, http.MethodGet, "/api/v1/admin/viewings", admin, ""), 500)
}

func TestViewingPostgresBookingAndAvailability(t *testing.T) {
	db, router := viewingPostgresRouter(t, true)
	user := applicationToken(t, 11, "user")
	other := applicationToken(t, 22, "user")
	admin := applicationToken(t, 33, "admin")
	prop := property.Property{Title: "Viewing test", Address: "Test address", Price: 100}
	if err := db.Create(&prop).Error; err != nil {
		t.Fatal(err)
	}
	loc := time.FixedZone("Astana", 5*60*60)
	tomorrow := time.Now().In(loc).AddDate(0, 0, 2)
	start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 10, 0, 0, 0, loc)
	body := fmt.Sprintf(`{"property_id":%d,"starts_at":%q,"user_id":22,"status":"completed"}`, prop.ID, start.Format(time.RFC3339))
	availabilityPath := fmt.Sprintf("/api/v1/properties/%d/available-slots?date=%s", prop.ID, start.Format("2006-01-02"))
	checkAvailability := func(want bool) {
		t.Helper()
		response := applicationRequest(router, http.MethodGet, availabilityPath, user, "")
		requireApplicationStatus(t, response, 200)
		var result viewing.AvailableSlotsResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.PropertyID != prop.ID || result.Date != start.Format("2006-01-02") || len(result.Slots) != 18 {
			t.Fatalf("unexpected availability: %+v", result)
		}
		if result.Slots[0].Available != want {
			t.Fatalf("first slot availability = %v, want %v", result.Slots[0].Available, want)
		}
		if !result.Slots[0].StartsAt.Equal(start) || !result.Slots[17].StartsAt.Equal(start.Add(8*time.Hour+30*time.Minute)) {
			t.Fatal("incorrect opening/closing slots")
		}
		for _, slot := range result.Slots {
			if !strings.HasSuffix(slot.StartsAt.Format(time.RFC3339), "Z") {
				t.Fatalf("non-UTC slot: %v", slot.StartsAt)
			}
		}
		if !strings.Contains(response.Body.String(), `Z"`) || strings.Contains(response.Body.String(), "+05:00") {
			t.Fatal("slot JSON is not UTC")
		}
	}
	checkAvailability(true)
	response := applicationRequest(router, http.MethodPost, "/api/v1/viewings", user, body)
	requireApplicationStatus(t, response, 201)
	created := decodeViewing(t, response.Body.Bytes())
	if created.UserID != 11 || created.Status != viewing.ViewingStatusPending || !created.StartsAt.Equal(start) {
		t.Fatalf("ownership/status override: %+v", created)
	}
	checkAvailability(false)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPost, "/api/v1/viewings", other, body), 409)
	path := fmt.Sprintf("/api/v1/admin/viewings/%d/status", created.ID)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, path, admin, `{"status":"confirmed"}`), 200)
	checkAvailability(false)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPost, "/api/v1/viewings", user, body), 409)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, path, admin, `{"status":"cancelled"}`), 200)
	checkAvailability(true)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPost, "/api/v1/viewings", other, body), 201)
	// The final 18:30 start is bookable too.
	lastBody := fmt.Sprintf(`{"property_id":%d,"starts_at":%q}`, prop.ID, start.Add(8*time.Hour+30*time.Minute).UTC().Format(time.RFC3339))
	lastResponse := applicationRequest(router, http.MethodPost, "/api/v1/viewings", user, lastBody)
	requireApplicationStatus(t, lastResponse, 201)
	last := decodeViewing(t, lastResponse.Body.Bytes())
	lastStatusPath := fmt.Sprintf("/api/v1/admin/viewings/%d/status", last.ID)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, lastStatusPath, admin, `{"status":"confirmed"}`), 200)
	requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, lastStatusPath, admin, `{"status":"completed"}`), 200)
	availableResponse := applicationRequest(router, http.MethodGet, availabilityPath, user, "")
	requireApplicationStatus(t, availableResponse, 200)
	var available viewing.AvailableSlotsResponse
	if err := json.Unmarshal(availableResponse.Body.Bytes(), &available); err != nil {
		t.Fatal(err)
	}
	if len(available.Slots) != 18 || !available.Slots[17].Available {
		t.Fatal("completed slot still occupied")
	}
	requireApplicationStatus(t, applicationRequest(router, http.MethodPost, "/api/v1/viewings", user, lastBody), 201)
	missingBody := fmt.Sprintf(`{"property_id":999999,"starts_at":%q}`, start.Format(time.RFC3339))
	requireApplicationStatus(t, applicationRequest(router, http.MethodPost, "/api/v1/viewings", user, missingBody), 404)
	requireApplicationStatus(t, applicationRequest(router, http.MethodGet, "/api/v1/properties/999999/available-slots?date="+start.Format("2006-01-02"), user, ""), 404)
}

func TestViewingPostgresConcurrency(t *testing.T) {
	db, router := viewingPostgresRouter(t, true)
	prop := property.Property{Title: "Concurrent test", Address: "Test", Price: 100}
	if err := db.Create(&prop).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().AddDate(0, 0, 2)
	const attempts = 12
	results := make(chan error, attempts)
	ready := make(chan struct{})
	var wg sync.WaitGroup
	repo := viewing.NewViewingRepository(db)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			results <- repo.Create(context.Background(), &viewing.Viewing{UserID: 11, PropertyID: prop.ID, StartsAt: start, Status: viewing.ViewingStatusPending})
		}()
	}
	close(ready)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, viewing.ErrSlotTaken) {
			conflict++
		} else {
			t.Errorf("unexpected create error: %v", err)
		}
	}
	if success != 1 || conflict != attempts-1 {
		t.Fatalf("booking successes=%d conflicts=%d", success, conflict)
	}
	var row viewing.Viewing
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	admin := applicationToken(t, 33, "admin")
	for _, target := range []string{"confirmed", "completed"} {
		codes := make(chan int, attempts)
		ready := make(chan struct{})
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-ready
				response := applicationRequest(router, http.MethodPatch, fmt.Sprintf("/api/v1/admin/viewings/%d/status", row.ID), admin, fmt.Sprintf(`{"status":%q}`, target))
				codes <- response.Code
			}()
		}
		close(ready)
		wg.Wait()
		close(codes)
		success, conflict = 0, 0
		for code := range codes {
			if code == 200 {
				success++
			} else if code == 409 {
				conflict++
			} else {
				t.Errorf("unexpected update status %d", code)
			}
		}
		if success != 1 || conflict != attempts-1 {
			t.Fatalf("%s: successes=%d conflicts=%d", target, success, conflict)
		}
		var stored viewing.Viewing
		if err := db.First(&stored, row.ID).Error; err != nil {
			t.Fatal(err)
		}
		if string(stored.Status) != target {
			t.Fatalf("stored status = %s, want %s", stored.Status, target)
		}
	}
}

func TestViewingPostgresMigrationPreservesDuplicates(t *testing.T) {
	db, _ := viewingPostgresRouter(t, false)
	start := time.Now().UTC()
	rows := []viewing.Viewing{
		{UserID: 11, PropertyID: 1, StartsAt: start, Status: viewing.ViewingStatusPending},
		{UserID: 22, PropertyID: 1, StartsAt: start, Status: viewing.ViewingStatusConfirmed},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := viewing.EnsureActiveSlotIndex(db); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	var stored []viewing.Viewing
	if err := db.Order("id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0].Status != rows[0].Status || stored[1].Status != rows[1].Status {
		t.Fatalf("migration changed existing data: %+v", stored)
	}
	if err := db.Model(&rows[0]).Update("status", viewing.ViewingStatusCancelled).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := viewing.EnsureActiveSlotIndex(db); err != nil {
			t.Fatal(err)
		}
	}
	if err := viewing.NewViewingRepository(db).Create(context.Background(), &viewing.Viewing{UserID: 33, PropertyID: 1, StartsAt: start, Status: viewing.ViewingStatusPending}); !errors.Is(err, viewing.ErrSlotTaken) {
		t.Fatalf("confirmed slot was not protected: %v", err)
	}
}
