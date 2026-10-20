package application

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ApplicationHandler struct {
	service *ApplicationService
}

func NewApplicationHandler(service *ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: service}
}

func (h *ApplicationHandler) UpdateStatus(c *gin.Context) {
	// PostgreSQL IDs are signed bigint values, even though the model uses uint.
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize-1)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application ID"})
		return
	}
	var req UpdateApplicationStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	app, err := h.service.UpdateStatus(c.Request.Context(), uint(id), req.Status)
	switch {
	case errors.Is(err, ErrInvalidStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
	case errors.Is(err, ErrInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update application status"})
	default:
		c.JSON(http.StatusOK, app)
	}
}

func (h *ApplicationHandler) Create(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app, err := h.service.Create(c.Request.Context(), userID, req)
	switch {
	case errors.Is(err, ErrPropertyNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAlreadyApplied):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create application"})
	default:
		c.JSON(http.StatusCreated, app)
	}
}

func (h *ApplicationHandler) GetMine(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	apps, err := h.service.GetMine(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch applications"})
		return
	}
	c.JSON(http.StatusOK, apps)
}

func (h *ApplicationHandler) GetAll(c *gin.Context) {
	apps, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch applications"})
		return
	}
	c.JSON(http.StatusOK, apps)
}
