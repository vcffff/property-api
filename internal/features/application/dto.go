package application

type CreateApplicationRequest struct {
	PropertyID uint `json:"property_id" binding:"required"`
}

type UpdateApplicationStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
}

type ApplicationResponse struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	PropertyID uint   `json:"property_id"`
	Status     string `json:"status"`
}
