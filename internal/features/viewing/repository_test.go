package viewing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetBookedViewingsRejectsInvalidInput(t *testing.T) {
	repo := NewViewingRepository(nil)
	now := time.Now()
	for _, tc := range []struct {
		name     string
		id       uint
		from, to time.Time
		want     error
	}{
		{"zero property ID", 0, now, now.Add(time.Hour), ErrInvalidPropertyID},
		{"equal boundaries", 1, now, now, ErrInvalidRange},
		{"reversed boundaries", 1, now.Add(time.Hour), now, ErrInvalidRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.GetBookedViewings(context.Background(), tc.id, tc.from, tc.to); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := repo.GetByUserID(context.Background(), 0); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("zero user ID: got %v", err)
	}
}
