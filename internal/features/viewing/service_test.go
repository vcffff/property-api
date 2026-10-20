package viewing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetAvailableSlotsInvalidDate(t *testing.T) {
	service := NewViewingService(nil)
	for _, date := range []string{"", "2026-02-30", "24-10-2026", "2026-10-24T00:00:00Z", "0000-01-01"} {
		if _, err := service.GetAvailableSlots(context.Background(), 1, date); !errors.Is(err, ErrInvalidSlot) {
			t.Errorf("date %q: got %v, want ErrInvalidSlot", date, err)
		}
	}
}

func TestViewingServiceRejectsInvalidIDs(t *testing.T) {
	service := NewViewingService(nil)
	ctx := context.Background()
	if _, err := service.GetAvailableSlots(ctx, 0, "2026-10-24"); !errors.Is(err, ErrInvalidPropertyID) {
		t.Fatalf("zero property ID: got %v", err)
	}
	if _, err := service.Book(ctx, 1, CreateViewingRequest{}); !errors.Is(err, ErrInvalidPropertyID) {
		t.Fatalf("booking zero property ID: got %v", err)
	}
	if _, err := service.Book(ctx, 0, CreateViewingRequest{PropertyID: 1}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("booking zero user ID: got %v", err)
	}
	if _, err := service.GetMine(ctx, 0); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("listing zero user ID: got %v", err)
	}
}

func TestUpdateViewingStatusValidation(t *testing.T) {
	service := NewViewingService(nil)
	if _, err := service.UpdateStatus(context.Background(), 0, ViewingStatusConfirmed); !errors.Is(err, ErrInvalidViewingID) {
		t.Fatalf("zero ID: %v", err)
	}
	for _, status := range []ViewingStatus{"", "approved", "Confirmed"} {
		if _, err := service.UpdateStatus(context.Background(), 1, status); !errors.Is(err, ErrInvalidStatus) {
			t.Errorf("status %q: %v", status, err)
		}
	}
}

func TestBookRejectsInvalidSlots(t *testing.T) {
	service := NewViewingService(nil)
	loc := time.FixedZone("Astana", 5*60*60)
	future := time.Now().In(loc).AddDate(0, 0, 2)
	for _, tc := range []struct {
		name                       string
		hour, minute, second, nano int
	}{
		{"before opening", 9, 30, 0, 0},
		{"at closing", 19, 0, 0, 0},
		{"off grid", 15, 15, 0, 0},
		{"seconds", 15, 0, 1, 0},
		{"nanoseconds", 15, 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Date(future.Year(), future.Month(), future.Day(), tc.hour, tc.minute, tc.second, tc.nano, loc)
			if _, err := service.Book(context.Background(), 11, CreateViewingRequest{PropertyID: 1, StartsAt: start}); !errors.Is(err, ErrInvalidSlot) {
				t.Fatalf("got %v, want ErrInvalidSlot", err)
			}
		})
	}
	if _, err := service.Book(context.Background(), 11, CreateViewingRequest{PropertyID: 1, StartsAt: time.Now().Add(-time.Hour)}); !errors.Is(err, ErrPastViewing) {
		t.Fatalf("past booking: %v", err)
	}
}

func TestBuildAvailableSlots(t *testing.T) {
	loc := time.FixedZone("Astana", 5*60*60)
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, loc)
	now := day.Add(10 * time.Hour)
	// 05:30 UTC is 10:30 in Astana. Midnight falls on the previous UTC day.
	booked := []time.Time{time.Date(2026, 10, 24, 5, 30, 0, 0, time.UTC)}
	slots := buildAvailableSlots(day, booked, now)
	if len(slots) != 18 {
		t.Fatalf("slots = %d, want 18", len(slots))
	}
	for i, slot := range slots {
		wantStart := day.Add(10*time.Hour + time.Duration(i)*30*time.Minute)
		if !slot.StartsAt.Equal(wantStart) {
			t.Errorf("slot %d starts at %v, want %v", i, slot.StartsAt, wantStart)
		}
		if _, offset := slot.StartsAt.Zone(); offset != 0 {
			t.Errorf("slot %d timezone offset = %d", i, offset)
		}
		if wantAvailable := i >= 2; slot.Available != wantAvailable {
			t.Errorf("slot %d available = %v, want %v", i, slot.Available, wantAvailable)
		}
	}
}

func TestBuildAvailableSlotsPastAndFutureDays(t *testing.T) {
	loc := time.FixedZone("Astana", 5*60*60)
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, loc)
	for _, tc := range []struct {
		name      string
		now       time.Time
		available bool
	}{
		{"future day", day.AddDate(0, 0, -1), true},
		{"past day", day.AddDate(0, 0, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, slot := range buildAvailableSlots(day, nil, tc.now) {
				if slot.Available != tc.available {
					t.Errorf("slot %d available = %v, want %v", i, slot.Available, tc.available)
				}
			}
		})
	}
}
