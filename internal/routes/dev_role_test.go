package routes

import (
	"net/http"
	"testing"
)

func TestDevRoleRouteDisabled(t *testing.T) {
	for _, tc := range []struct{ name, env, enabled string }{
		{"default", "", ""},
		{"flag alone", "", "true"},
		{"development alone", "development", ""},
		{"explicitly disabled", "development", "false"},
		{"production", "production", "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("ENABLE_DEV_ROLE_ENDPOINT", tc.enabled)
			response := applicationRequest(applicationRouter(nil), http.MethodPatch, "/api/v1/dev/users/me/role", "", `{"role":"admin"}`)
			requireApplicationStatus(t, response, http.StatusNotFound)
		})
	}
}

func TestDevRoleAuthorizationAndValidation(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ENABLE_DEV_ROLE_ENDPOINT", "true")
	router := applicationRouter(nil)
	user := applicationToken(t, 11, "user")
	for _, tc := range []struct {
		name, token, body string
		status            int
	}{
		{"missing JWT", "", `{"role":"admin"}`, 401},
		{"invalid JWT", "invalid", `{"role":"admin"}`, 401},
		{"missing role", user, `{}`, 400},
		{"invalid role", user, `{"role":"superadmin"}`, 400},
		{"wrong type", user, `{"role":1}`, 400},
		{"malformed JSON", user, `{"role":`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := applicationRequest(router, http.MethodPatch, "/api/v1/dev/users/me/role", tc.token, tc.body)
			requireApplicationStatus(t, response, tc.status)
		})
	}
}
