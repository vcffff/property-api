package application

import (
	"context"
	"errors"
	"testing"
)

func TestUpdateStatusRejectsInvalidTargets(t *testing.T) {
	service := NewApplicationService(nil)
	for _, status := range []string{"", "pending", "cancelled", "Approved"} {
		t.Run(status, func(t *testing.T) {
			if _, err := service.UpdateStatus(context.Background(), 1, status); !errors.Is(err, ErrInvalidStatus) {
				t.Fatalf("error = %v, want ErrInvalidStatus", err)
			}
		})
	}
}
