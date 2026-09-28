package property

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// patchDB records SQL while providing PostgreSQL-style RETURNING rows without a live database.
type patchDB struct {
	property Property
	missing  bool
	fail     bool
	query    string
	deleted  bool
}

func (d *patchDB) Connect(context.Context) (driver.Conn, error) { return d, nil }
func (d *patchDB) Driver() driver.Driver                        { return d }
func (d *patchDB) Open(string) (driver.Conn, error)             { return d, nil }
func (d *patchDB) Prepare(string) (driver.Stmt, error)          { return nil, errors.New("unexpected Prepare") }
func (d *patchDB) Close() error                                 { return nil }
func (d *patchDB) Begin() (driver.Tx, error)                    { return nil, errors.New("unexpected transaction") }
func (d *patchDB) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	d.query = query
	if d.fail {
		return nil, errors.New("database unavailable")
	}
	if d.missing {
		return &patchRows{}, nil
	}
	if strings.HasPrefix(query, "UPDATE") {
		// GORM sorts the update map by column name.
		i := 0
		for _, column := range []string{"address", "price", "title"} {
			if strings.Contains(query, `"`+column+`"=`) {
				switch column {
				case "address":
					d.property.Address = args[i].Value.(string)
				case "price":
					d.property.Price = int(args[i].Value.(int64))
				case "title":
					d.property.Title = args[i].Value.(string)
				}
				i++
			}
		}
	}
	return &patchRows{property: &d.property}, nil
}

func (d *patchDB) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	d.query = query
	if d.fail {
		return nil, errors.New("database unavailable")
	}
	if d.missing || d.deleted {
		return driver.RowsAffected(0), nil
	}
	d.deleted = true
	return driver.RowsAffected(1), nil
}

type patchRows struct {
	property *Property
	read     bool
}

func (*patchRows) Columns() []string { return []string{"id", "title", "address", "price"} }
func (*patchRows) Close() error      { return nil }
func (r *patchRows) Next(dest []driver.Value) error {
	if r.read || r.property == nil {
		return io.EOF
	}
	r.read = true
	dest[0], dest[1], dest[2], dest[3] = int64(r.property.ID), r.property.Title, r.property.Address, int64(r.property.Price)
	return nil
}

func TestPatchProperty(t *testing.T) {
	initial := Property{ID: 1, Title: "Apartment", Address: "Main Street", Price: 100}
	for _, tc := range []struct {
		name, body    string
		missing, fail bool
		status        int
		want          Property
	}{
		{name: "price only", body: `{"price":45000000}`, status: 200, want: Property{1, "Apartment", "Main Street", 45000000}},
		{name: "explicit zero", body: `{"price":0}`, status: 200, want: Property{1, "Apartment", "Main Street", 0}},
		{name: "title only", body: `{"title":"House"}`, status: 200, want: Property{1, "House", "Main Street", 100}},
		{name: "empty address", body: `{"address":""}`, status: 200, want: Property{1, "Apartment", "", 100}},
		{name: "empty patch", body: `{}`, status: 200, want: initial},
		{name: "missing", body: `{"price":1}`, missing: true, status: 404},
		{name: "missing empty patch", body: `{}`, missing: true, status: 404},
		{name: "database failure", body: `{"price":1}`, fail: true, status: 500},
		{name: "invalid json", body: `{"price":`, status: 400},
		{name: "wrong type", body: `{"price":"abc"}`, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &patchDB{property: initial, missing: tc.missing, fail: tc.fail}
			conn := sql.OpenDB(fake)
			t.Cleanup(func() { conn.Close() })
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
			if err != nil {
				t.Fatal(err)
			}
			handler := NewPropertyHandler(NewPropertyService(NewPropertyRepository(db)))
			router := gin.New()
			router.PATCH("/properties/:id", handler.Update)
			req := httptest.NewRequest(http.MethodPatch, "/properties/1", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tc.status, recorder.Body.String())
			}
			if tc.status == 200 {
				var got Property
				if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got != tc.want {
					t.Errorf("property = %+v, want %+v", got, tc.want)
				}
			}
			if tc.name == "price only" {
				if !strings.Contains(fake.query, `SET "price"=`) || strings.Contains(fake.query, `"title"=`) || strings.Contains(fake.query, `"address"=`) || !strings.Contains(fake.query, "RETURNING") {
					t.Errorf("expected price-only UPDATE with RETURNING, got %s", fake.query)
				}
			}
		})
	}
}
