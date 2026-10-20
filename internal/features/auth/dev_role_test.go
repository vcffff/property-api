package auth

import (
	"context"
	"testing"
)

func TestUpdateDevRoleRejectsInvalidRole(t *testing.T) {
	service := NewServiceAuth(nil, nil)
	for _, role := range []string{"", "superadmin", "Admin"} {
		if _, err := service.UpdateDevRole(context.Background(), 1, role); err == nil {
			t.Fatalf("role %q must be rejected before accessing the database", role)
		}
	}
}
