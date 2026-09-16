package property

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PropertyHandler struct {
	service *PropertyService
}

func NewPropertyHandler(service *PropertyService) *PropertyHandler {
	return &PropertyHandler{
		service: service,
	}
}

func (h *PropertyHandler) Create(c *gin.Context) {
	var property Property
	if err := c.ShouldBindJSON(&property); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateProperty(&property); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, property)
}

func (h *PropertyHandler) GetAll(c *gin.Context) {
	properties, err := h.service.GetAllProperties()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, properties)
}

func (h *PropertyHandler) GetByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid property ID"})
		return
	}

	property, err := h.service.GetPropertyByID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if property == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "property not found"})
		return
	}

	c.JSON(http.StatusOK, property)
}

func (h *PropertyHandler) Update(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid property ID"})
		return
	}

	var updateReq UpdatePropertyRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedProperty, err := h.service.UpdateProperty(uint(id), &updateReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedProperty)
}
