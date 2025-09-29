package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"gorm.io/gorm"
)

// ObservationHandler handles HTTP requests for observation resources
type ObservationHandler struct {
	db        *gorm.DB
	validator *validator.Validate
}

// NewObservationHandler creates a new observation handler
func NewObservationHandler(db *gorm.DB) *ObservationHandler {
	return &ObservationHandler{
		db:        db,
		validator: validator.New(),
	}
}

// CreateObservation creates a new observation
// @Summary Create a new observation
// @Description Create a new lab result observation
// @Tags observations
// @Accept json
// @Produce json
// @Param observation body models.Observation true "Observation data"
// @Success 201 {object} models.Observation
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/observations [post]
func (h *ObservationHandler) CreateObservation(c *gin.Context) {
	var observation models.Observation

	if err := c.ShouldBindJSON(&observation); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
			Code:    "INVALID_REQUEST_BODY",
		})
		return
	}

	if err := h.validator.Struct(observation); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
			Code:    "VALIDATION_FAILED",
		})
		return
	}

	// Validate that the referenced patient exists
	if observation.Subject.Reference != "" {
		patientID := strings.TrimPrefix(observation.Subject.Reference, "Patient/")
		var patient models.Patient
		if err := h.db.Where("id = ?", patientID).First(&patient).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error: "Referenced patient not found",
					Code:  "PATIENT_NOT_FOUND",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to validate patient reference",
				Message: err.Error(),
				Code:    "DATABASE_ERROR",
			})
			return
		}
	}

	// Set created by user
	if userID, exists := auth.GetUserID(c); exists {
		observation.CreatedBy = userID
	}

	if err := h.db.Create(&observation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, observation)
}

// GetObservations retrieves observations with pagination and filtering
// @Summary Get observations
// @Description Get a list of observations with pagination and optional filtering
// @Tags observations
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 10, max: 100)"
// @Param patient query string false "Filter by patient ID"
// @Param status query string false "Filter by status"
// @Param category query string false "Filter by category"
// @Param code query string false "Filter by observation code"
// @Param from query string false "Filter by effective date from (ISO 8601)"
// @Param to query string false "Filter by effective date to (ISO 8601)"
// @Success 200 {object} PaginatedResponse{data=[]models.Observation}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/observations [get]
func (h *ObservationHandler) GetObservations(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	patientID := strings.TrimSpace(c.Query("patient"))
	status := strings.TrimSpace(c.Query("status"))
	category := strings.TrimSpace(c.Query("category"))
	code := strings.TrimSpace(c.Query("code"))
	fromDate := strings.TrimSpace(c.Query("from"))
	toDate := strings.TrimSpace(c.Query("to"))

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var observations []models.Observation
	query := h.db.Model(&models.Observation{})

	// Apply filters
	if patientID != "" {
		patientRef := "Patient/" + patientID
		query = query.Where("subject->>'reference' = ?", patientRef)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if category != "" {
		query = query.Where("category::text ILIKE ?", "%"+category+"%")
	}

	if code != "" {
		query = query.Where("code->>'text' ILIKE ? OR code->'coding'->0->>'code' ILIKE ? OR code->'coding'->0->>'display' ILIKE ?",
			"%"+code+"%", "%"+code+"%", "%"+code+"%")
	}

	if fromDate != "" {
		query = query.Where("effective_date_time >= ?", fromDate)
	}

	if toDate != "" {
		query = query.Where("effective_date_time <= ?", toDate)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to count observations",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Get observations with pagination
	offset := (page - 1) * limit
	if err := query.Order("effective_date_time DESC").Offset(offset).Limit(limit).Find(&observations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch observations",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	response := PaginatedResponse{
		Data:       observations,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: (total + int64(limit) - 1) / int64(limit),
	}

	c.JSON(http.StatusOK, response)
}

// GetObservation retrieves a specific observation by ID
// @Summary Get observation by ID
// @Description Get a specific observation by its ID
// @Tags observations
// @Accept json
// @Produce json
// @Param id path string true "Observation ID"
// @Success 200 {object} models.Observation
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/observations/{id} [get]
func (h *ObservationHandler) GetObservation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Observation ID is required",
			Code:  "MISSING_OBSERVATION_ID",
		})
		return
	}

	var observation models.Observation
	if err := h.db.Where("id = ?", id).First(&observation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Observation not found",
				Code:  "OBSERVATION_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, observation)
}

// UpdateObservation updates an existing observation
// @Summary Update observation
// @Description Update an existing observation record
// @Tags observations
// @Accept json
// @Produce json
// @Param id path string true "Observation ID"
// @Param observation body models.Observation true "Updated observation data"
// @Success 200 {object} models.Observation
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/observations/{id} [put]
func (h *ObservationHandler) UpdateObservation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Observation ID is required",
			Code:  "MISSING_OBSERVATION_ID",
		})
		return
	}

	var observation models.Observation
	if err := h.db.Where("id = ?", id).First(&observation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Observation not found",
				Code:  "OBSERVATION_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	var updateData models.Observation
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
			Code:    "INVALID_REQUEST_BODY",
		})
		return
	}

	if err := h.validator.Struct(updateData); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
			Code:    "VALIDATION_FAILED",
		})
		return
	}

	// Validate patient reference if changed
	if updateData.Subject.Reference != "" && updateData.Subject.Reference != observation.Subject.Reference {
		patientID := strings.TrimPrefix(updateData.Subject.Reference, "Patient/")
		var patient models.Patient
		if err := h.db.Where("id = ?", patientID).First(&patient).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error: "Referenced patient not found",
					Code:  "PATIENT_NOT_FOUND",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to validate patient reference",
				Message: err.Error(),
				Code:    "DATABASE_ERROR",
			})
			return
		}
	}

	// Preserve ID and audit fields
	updateData.ID = id
	updateData.CreatedAt = observation.CreatedAt
	updateData.CreatedBy = observation.CreatedBy

	if err := h.db.Model(&observation).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Fetch updated observation
	if err := h.db.Where("id = ?", id).First(&observation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, observation)
}

// DeleteObservation deletes an observation
// @Summary Delete observation
// @Description Delete an observation record (admin only)
// @Tags observations
// @Accept json
// @Produce json
// @Param id path string true "Observation ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/observations/{id} [delete]
func (h *ObservationHandler) DeleteObservation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Observation ID is required",
			Code:  "MISSING_OBSERVATION_ID",
		})
		return
	}

	// Check if observation exists
	var observation models.Observation
	if err := h.db.Where("id = ?", id).First(&observation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Observation not found",
				Code:  "OBSERVATION_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Delete the observation
	if err := h.db.Delete(&observation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete observation",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPatientObservations retrieves all observations for a specific patient
// @Summary Get patient observations
// @Description Get all observations for a specific patient
// @Tags observations
// @Accept json
// @Produce json
// @Param patientId path string true "Patient ID"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 10, max: 100)"
// @Param status query string false "Filter by status"
// @Param category query string false "Filter by category"
// @Success 200 {object} PaginatedResponse{data=[]models.Observation}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/patients/{patientId}/observations [get]
func (h *ObservationHandler) GetPatientObservations(c *gin.Context) {
	patientID := c.Param("patientId")
	if patientID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Patient ID is required",
			Code:  "MISSING_PATIENT_ID",
		})
		return
	}

	// Verify patient exists
	var patient models.Patient
	if err := h.db.Where("id = ?", patientID).First(&patient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Patient not found",
				Code:  "PATIENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to verify patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := strings.TrimSpace(c.Query("status"))
	category := strings.TrimSpace(c.Query("category"))

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var observations []models.Observation
	patientRef := "Patient/" + patientID
	query := h.db.Model(&models.Observation{}).Where("subject->>'reference' = ?", patientRef)

	// Apply additional filters
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if category != "" {
		query = query.Where("category::text ILIKE ?", "%"+category+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to count observations",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Get observations with pagination
	offset := (page - 1) * limit
	if err := query.Order("effective_date_time DESC").Offset(offset).Limit(limit).Find(&observations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch observations",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	response := PaginatedResponse{
		Data:       observations,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: (total + int64(limit) - 1) / int64(limit),
	}

	c.JSON(http.StatusOK, response)
}
