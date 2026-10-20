package viewing

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ViewingHandler struct {
	service *ViewingService
}

func NewViewingHandler(service *ViewingService) *ViewingHandler {
	return &ViewingHandler{service: service}
}

func (h *ViewingHandler) GetAvailableSlots(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize-1)
	if err != nil || propertyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid property ID"})
		return
	}
	result, err := h.service.GetAvailableSlots(c.Request.Context(), uint(propertyID), c.Query("date"))
	switch {
	case errors.Is(err, ErrInvalidPropertyID), errors.Is(err, ErrInvalidRange):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidSlot):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date: expected YYYY-MM-DD"})
	case errors.Is(err, ErrPropertyGone):
		c.JSON(http.StatusNotFound, gin.H{"error": "property not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, result)
	}
}

func (h *ViewingHandler) Book(c *gin.Context) {
	var req CreateViewingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	result, err := h.service.Book(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidPropertyID):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPastViewing):
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot book a viewing in the past"})
		case errors.Is(err, ErrInvalidSlot):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid viewing time slot"})
		case errors.Is(err, ErrPropertyGone):
			c.JSON(http.StatusNotFound, gin.H{"error": "property not found"})
		case errors.Is(err, ErrSlotTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "viewing slot already booked"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *ViewingHandler) GetMine(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	viewings, err := h.service.GetMine(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch viewings"})
		return
	}
	c.JSON(http.StatusOK, viewings)
}

func (h *ViewingHandler) GetAll(c *gin.Context) {
	viewings, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch viewings"})
		return
	}
	c.JSON(http.StatusOK, viewings)
}

func (h *ViewingHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize-1)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid viewing ID"})
		return
	}
	var req UpdateViewingStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.service.UpdateStatus(c.Request.Context(), uint(id), req.Status)
	switch {
	case errors.Is(err, ErrInvalidStatus), errors.Is(err, ErrInvalidViewingID):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "viewing not found"})
	case errors.Is(err, ErrInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update viewing status"})
	default:
		c.JSON(http.StatusOK, updated)
	}
}
