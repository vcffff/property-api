package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dev/api-task-manager/internal/features/application"
	"dev/api-task-manager/internal/features/auth"
	"dev/api-task-manager/internal/features/property"
	"dev/api-task-manager/internal/features/viewing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func applicationToken(t *testing.T, userID uint, role string) string {
	t.Helper()
	token, err := auth.GenerateToken(userID, role)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func applicationRouter(service *application.ApplicationService) *gin.Engine {
	router := gin.New()
	SetUpRoutes(router, &auth.Handler{}, property.NewPropertyHandler(nil), application.NewApplicationHandler(service), viewing.NewViewingHandler(viewing.NewViewingService(nil)))
	return router
}

func applicationRequest(router *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func requireApplicationStatus(t *testing.T, response *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if response.Code != expected {
		t.Fatalf("status = %d, want %d: %s", response.Code, expected, response.Body.String())
	}
}

func TestApplicationAccessAndValidation(t *testing.T) {
	user := applicationToken(t, 11, "user")
	// Nil persistence ensures rejected requests cannot reach the service.
	router := applicationRouter(nil)
	for _, tc := range []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"create without JWT", "POST", "/api/v1/applications", "", `{"property_id":1}`, 401},
		{"create invalid JWT", "POST", "/api/v1/applications", "invalid", `{"property_id":1}`, 401},
		{"my list without JWT", "GET", "/api/v1/applications/me", "", "", 401},
		{"my list invalid JWT", "GET", "/api/v1/applications/me", "invalid", "", 401},
		{"admin list without JWT", "GET", "/api/v1/admin/applications", "", "", 401},
		{"admin list invalid JWT", "GET", "/api/v1/admin/applications", "invalid", "", 401},
		{"admin list as user", "GET", "/api/v1/admin/applications", user, "", 403},
		{"missing property", "POST", "/api/v1/applications", user, `{}`, 400},
		{"null property", "POST", "/api/v1/applications", user, `{"property_id":null}`, 400},
		{"zero property", "POST", "/api/v1/applications", user, `{"property_id":0}`, 400},
		{"negative property", "POST", "/api/v1/applications", user, `{"property_id":-1}`, 400},
		{"string property", "POST", "/api/v1/applications", user, `{"property_id":"1"}`, 400},
		{"fractional property", "POST", "/api/v1/applications", user, `{"property_id":1.5}`, 400},
		{"overflow property", "POST", "/api/v1/applications", user, `{"property_id":18446744073709551616}`, 400},
		{"malformed JSON", "POST", "/api/v1/applications", user, `{"property_id":`, 400},
		{"empty body", "POST", "/api/v1/applications", user, "", 400},
		{"array body", "POST", "/api/v1/applications", user, `[]`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireApplicationStatus(t, applicationRequest(router, tc.method, tc.path, tc.token, tc.body), tc.status)
		})
	}
}

// Every pooled connection receives its own schema search_path, keeping these
// opt-in integration tests isolated from the database's existing tables.
func applicationDatabase(t *testing.T, withActiveIndex bool) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("APPLICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set APPLICATION_TEST_DATABASE_URL to run PostgreSQL application tests")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect to test PostgreSQL: %v", err)
	}
	schema := pgx.Identifier{fmt.Sprintf("application_test_%d_%d", os.Getpid(), time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close(ctx)
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Errorf("close test PostgreSQL connection: %v", err)
		}
	})
	config = config.Copy()
	config.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*config)
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&property.Property{}, &application.Application{}); err != nil {
		t.Fatal(err)
	}
	if withActiveIndex {
		if err := application.EnsureActiveApplicationIndex(db); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func decodeApplicationList(t *testing.T, response *httptest.ResponseRecorder) []application.ApplicationResponse {
	t.Helper()
	requireApplicationStatus(t, response, http.StatusOK)
	var items []application.ApplicationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Fatalf("list must be a JSON array, got %s", response.Body.String())
	}
	return items
}

func TestApplicationsPostgresLifecycle(t *testing.T) {
	db := applicationDatabase(t, true)
	router := applicationRouter(application.NewApplicationService(application.NewApplicationRepository(db)))
	firstUser := applicationToken(t, 11, "user")
	secondUser := applicationToken(t, 22, "user")
	admin := applicationToken(t, 33, "admin")
	propertyRow := property.Property{Title: "Test property", Address: "Test address", Price: 100}
	if err := db.Create(&propertyRow).Error; err != nil {
		t.Fatal(err)
	}

	for _, list := range []struct{ path, token string }{
		{"/api/v1/applications/me", firstUser},
		{"/api/v1/admin/applications", admin},
	} {
		if items := decodeApplicationList(t, applicationRequest(router, "GET", list.path, list.token, "")); len(items) != 0 {
			t.Fatalf("expected empty list, got %+v", items)
		}
	}
	requireApplicationStatus(t, applicationRequest(router, "POST", "/api/v1/applications", firstUser, `{"property_id":999999}`), http.StatusNotFound)
	// Client-provided ownership and status must never override server values.
	body := fmt.Sprintf(`{"property_id":%d,"user_id":22,"status":"approved"}`, propertyRow.ID)
	response := applicationRequest(router, "POST", "/api/v1/applications", firstUser, body)
	requireApplicationStatus(t, response, http.StatusCreated)
	var created application.ApplicationResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.UserID != 11 || created.PropertyID != propertyRow.ID || created.Status != "pending" {
		t.Fatalf("unexpected create response: %+v", created)
	}
	var persisted application.Application
	if err := db.First(&persisted, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.UserID != 11 || persisted.PropertyID != propertyRow.ID || persisted.Status != "pending" || persisted.CreatedAt.IsZero() || persisted.UpdatedAt.IsZero() {
		t.Fatalf("unexpected stored application: %+v", persisted)
	}
	requireApplicationStatus(t, applicationRequest(router, "POST", "/api/v1/applications", firstUser, body), http.StatusConflict)
	if err := db.Model(&persisted).Update("status", "approved").Error; err != nil {
		t.Fatal(err)
	}
	requireApplicationStatus(t, applicationRequest(router, "POST", "/api/v1/applications", firstUser, body), http.StatusConflict)
	if err := db.Model(&persisted).Update("status", "rejected").Error; err != nil {
		t.Fatal(err)
	}
	requireApplicationStatus(t, applicationRequest(router, "POST", "/api/v1/applications", firstUser, body), http.StatusCreated)
	if items := decodeApplicationList(t, applicationRequest(router, "GET", "/api/v1/applications/me", secondUser, "")); len(items) != 0 {
		t.Fatalf("another user's list contains applications: %+v", items)
	}
	requireApplicationStatus(t, applicationRequest(router, "POST", "/api/v1/applications", secondUser, body), http.StatusCreated)
	requireApplicationStatus(t, applicationRequest(router, "POST", "/api/v1/applications", admin, body), http.StatusCreated)
	own := decodeApplicationList(t, applicationRequest(router, "GET", "/api/v1/applications/me", firstUser, ""))
	if len(own) != 2 || own[0].ID <= own[1].ID || own[0].Status != "pending" || own[1].Status != "rejected" {
		t.Fatalf("expected newest-first application history: %+v", own)
	}
	for _, item := range own {
		if item.UserID != 11 {
			t.Fatalf("ownership leak in my applications: %+v", own)
		}
	}
	all := decodeApplicationList(t, applicationRequest(router, "GET", "/api/v1/admin/applications", admin, ""))
	if len(all) != 4 || all[0].UserID != 33 || all[1].UserID != 22 || all[0].ID <= all[1].ID || all[1].ID <= all[2].ID || all[2].ID <= all[3].ID {
		t.Fatalf("expected all users' newest-first application history: %+v", all)
	}

	// An older ID with a later timestamp comes first; equal timestamps use ID.
	tiedTimestamp := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := db.Model(&application.Application{}).Where("id > ?", 0).Update("created_at", tiedTimestamp).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&application.Application{}).Where("id = ?", created.ID).Update("created_at", tiedTimestamp.Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	for _, list := range []struct {
		path, token string
		ids         []uint
	}{
		{"/api/v1/applications/me", firstUser, []uint{created.ID, own[0].ID}},
		{"/api/v1/admin/applications", admin, []uint{created.ID, all[0].ID, all[1].ID, own[0].ID}},
	} {
		items := decodeApplicationList(t, applicationRequest(router, "GET", list.path, list.token, ""))
		if len(items) != len(list.ids) {
			t.Fatalf("%s length = %d, want %d", list.path, len(items), len(list.ids))
		}
		for i, id := range list.ids {
			if items[i].ID != id {
				t.Fatalf("%s order = %+v, want IDs %v", list.path, items, list.ids)
			}
		}
	}
}

func TestApplicationsPostgresConcurrentCreate(t *testing.T) {
	db := applicationDatabase(t, true)
	repository := application.NewApplicationRepository(db)
	const attempts = 12
	start := make(chan struct{})
	results := make(chan error, attempts)
	var pending sync.WaitGroup
	for i := 0; i < attempts; i++ {
		pending.Add(1)
		go func() {
			defer pending.Done()
			<-start
			// Bypass the service's pre-check to exercise the database guarantee.
			results <- repository.Create(context.Background(), &application.Application{
				UserID: 11, PropertyID: 1, Status: "pending",
			})
		}()
	}
	close(start)
	pending.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, application.ErrAlreadyApplied):
			conflicts++
		default:
			t.Errorf("unexpected insert error: %v", err)
		}
	}
	if successes != 1 || conflicts != attempts-1 {
		t.Fatalf("successes = %d, conflicts = %d; want 1 and %d", successes, conflicts, attempts-1)
	}
	var count int64
	if err := db.Model(&application.Application{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stored applications = %d, want 1", count)
	}
	// The partial index also protects approved rows, while preserving history.
	if err := db.Model(&application.Application{}).Where("user_id = ?", 11).Update("status", "approved").Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(context.Background(), &application.Application{UserID: 11, PropertyID: 1, Status: "pending"}); !errors.Is(err, application.ErrAlreadyApplied) {
		t.Fatalf("approved duplicate error = %v, want ErrAlreadyApplied", err)
	}
	if err := repository.Create(context.Background(), &application.Application{UserID: 11, PropertyID: 1, Status: "rejected"}); err != nil {
		t.Fatalf("rejected history insert: %v", err)
	}
}

func TestApplicationsPostgresMigrationRejectsDuplicates(t *testing.T) {
	db := applicationDatabase(t, false)
	rows := []application.Application{
		{UserID: 11, PropertyID: 1, Status: "pending"},
		{UserID: 11, PropertyID: 1, Status: "approved"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	err := application.EnsureActiveApplicationIndex(db)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("expected actionable duplicate migration error, got %v", err)
	}
	var stored []application.Application
	if err := db.Order("id").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0].ID != rows[0].ID || stored[1].ID != rows[1].ID || stored[0].Status != "pending" || stored[1].Status != "approved" {
		t.Fatalf("migration modified existing duplicates: %+v", stored)
	}
}
