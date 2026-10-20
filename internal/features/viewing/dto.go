package viewing

import "time"

type CreateViewingRequest struct {
	PropertyID uint      `json:"property_id" binding:"required,gt=0"`
	StartsAt   time.Time `json:"starts_at" binding:"required"`
}

type AvailableSlot struct {
	StartsAt  time.Time `json:"starts_at"`
	Available bool      `json:"available"`
}

type AvailableSlotsResponse struct {
	PropertyID uint            `json:"property_id"`
	Date       string          `json:"date"`
	Slots      []AvailableSlot `json:"slots"`
}

type UpdateViewingStatusRequest struct {
	Status ViewingStatus `json:"status" binding:"required"`
}
