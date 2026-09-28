package property

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDeleteProperty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		missing bool
		fail    bool
		want    int
	}{
		{name: "existing property", want: http.StatusNoContent},
		{name: "missing property", missing: true, want: http.StatusNotFound},
		{name: "database failure", fail: true, want: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &patchDB{property: Property{ID: 4}, missing: tc.missing, fail: tc.fail}
			conn := sql.OpenDB(fake)
			t.Cleanup(func() { conn.Close() })
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
			if err != nil {
				t.Fatal(err)
			}
			handler := NewPropertyHandler(NewPropertyService(NewPropertyRepository(db)))
			router := gin.New()
			router.DELETE("/properties/:id", handler.Delete)

			request := func() *httptest.ResponseRecorder {
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/properties/4", nil))
				return recorder
			}
			response := request()
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.want, response.Body.String())
			}
			if !strings.Contains(fake.query, "DELETE FROM") {
				t.Errorf("expected DELETE query, got %q", fake.query)
			}
			if tc.want == http.StatusNoContent {
				if response.Body.Len() != 0 {
					t.Errorf("204 response has body: %s", response.Body.String())
				}
				if second := request(); second.Code != http.StatusNotFound {
					t.Errorf("second DELETE status = %d, want 404", second.Code)
				}
			}
		})
	}
}
