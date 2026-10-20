package routes

import (
	"net/http"
	"testing"
)

func TestViewingAdminAccessAndValidation(t *testing.T) {
	router := applicationRouter(nil)
	user := applicationToken(t, 11, "user")
	admin := applicationToken(t, 33, "admin")
	for _, path := range []string{"/api/v1/admin/viewings", "/api/v1/admin/viewings/1/status"} {
		method := http.MethodGet
		if path != "/api/v1/admin/viewings" {
			method = http.MethodPatch
		}
		for _, tc := range []struct {
			name, token string
			status      int
		}{
			{"missing JWT", "", 401}, {"invalid JWT", "invalid", 401}, {"user", user, 403},
		} {
			t.Run(method+" "+tc.name, func(t *testing.T) {
				requireApplicationStatus(t, applicationRequest(router, method, path, tc.token, `{"status":"confirmed"}`), tc.status)
			})
		}
	}
	for _, id := range []string{"abc", "0", "-1", "9223372036854775808"} {
		requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, "/api/v1/admin/viewings/"+id+"/status", admin, `{"status":"confirmed"}`), 400)
	}
	for _, body := range []string{"", `{}`, `{"status":null}`, `{"status":1}`, `{"status":"approved"}`, `{"status":"Confirmed"}`, `{"status":`} {
		requireApplicationStatus(t, applicationRequest(router, http.MethodPatch, "/api/v1/admin/viewings/1/status", admin, body), 400)
	}
}
