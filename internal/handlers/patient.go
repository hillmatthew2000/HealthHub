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

// PatientHandler handles HTTP requests for patient resources
type PatientHandler struct {
	db        *gorm.DB
	validator *validator.Validate
}

// NewPatientHandler creates a new patient handler
func NewPatientHandler(db *gorm.DB) *PatientHandler {
	return &PatientHandler{
		db:        db,
		validator: validator.New(),
	}
}

// CreatePatient creates a new patient
// @Summary Create a new patient
// @Description Create a new patient record
// @Tags patients
// @Accept json
// @Produce json
// @Param patient body models.Patient true "Patient data"
// @Success 201 {object} models.Patient
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/patients [post]
func (h *PatientHandler) CreatePatient(c *gin.Context) {
	var patient models.Patient

	if err := c.ShouldBindJSON(&patient); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
			Code:    "INVALID_REQUEST_BODY",
		})
		return
	}

	if err := h.validator.Struct(patient); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
			Code:    "VALIDATION_FAILED",
		})
		return
	}

	// Set created by user
	if userID, exists := auth.GetUserID(c); exists {
		patient.CreatedBy = userID
	}

	if err := h.db.Create(&patient).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusCreated, patient)
}

// GetPatients retrieves patients with pagination and filtering
// @Summary Get patients
// @Description Get a list of patients with pagination and optional filtering
// @Tags patients
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 10, max: 100)"
// @Param search query string false "Search term for name or contact info"
// @Param gender query string false "Filter by gender"
// @Param active query bool false "Filter by active status"
// @Success 200 {object} PaginatedResponse{data=[]models.Patient}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/patients [get]
func (h *PatientHandler) GetPatients(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := strings.TrimSpace(c.Query("search"))
	gender := strings.TrimSpace(c.Query("gender"))
	activeStr := strings.TrimSpace(c.Query("active"))

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var patients []models.Patient
	query := h.db.Model(&models.Patient{})

	// Apply filters
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name::text ILIKE ? OR telecom::text ILIKE ?", searchPattern, searchPattern)
	}

	if gender != "" {
		query = query.Where("gender = ?", gender)
	}

	if activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			query = query.Where("active = ?", active)
		}
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to count patients",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Get patients with pagination
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&patients).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch patients",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	response := PaginatedResponse{
		Data:       patients,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: (total + int64(limit) - 1) / int64(limit),
	}

	c.JSON(http.StatusOK, response)
}

// GetPatient retrieves a specific patient by ID
// @Summary Get patient by ID
// @Description Get a specific patient by their ID
// @Tags patients
// @Accept json
// @Produce json
// @Param id path string true "Patient ID"
// @Success 200 {object} models.Patient
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/patients/{id} [get]
func (h *PatientHandler) GetPatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Patient ID is required",
			Code:  "MISSING_PATIENT_ID",
		})
		return
	}

	var patient models.Patient
	if err := h.db.Where("id = ?", id).First(&patient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Patient not found",
				Code:  "PATIENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, patient)
}

// UpdatePatient updates an existing patient
// @Summary Update patient
// @Description Update an existing patient record
// @Tags patients
// @Accept json
// @Produce json
// @Param id path string true "Patient ID"
// @Param patient body models.Patient true "Updated patient data"
// @Success 200 {object} models.Patient
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/patients/{id} [put]
func (h *PatientHandler) UpdatePatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Patient ID is required",
			Code:  "MISSING_PATIENT_ID",
		})
		return
	}

	var patient models.Patient
	if err := h.db.Where("id = ?", id).First(&patient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Patient not found",
				Code:  "PATIENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	var updateData models.Patient
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

	// Preserve ID and audit fields
	updateData.ID = id
	updateData.CreatedAt = patient.CreatedAt
	updateData.CreatedBy = patient.CreatedBy

	if err := h.db.Model(&patient).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Fetch updated patient
	if err := h.db.Where("id = ?", id).First(&patient).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch updated patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, patient)
}

// DeletePatient deletes a patient
// @Summary Delete patient
// @Description Delete a patient record (admin only)
// @Tags patients
// @Accept json
// @Produce json
// @Param id path string true "Patient ID"
// @Success 204 "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /api/v1/patients/{id} [delete]
func (h *PatientHandler) DeletePatient(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Patient ID is required",
			Code:  "MISSING_PATIENT_ID",
		})
		return
	}

	// Check if patient exists
	var patient models.Patient
	if err := h.db.Where("id = ?", id).First(&patient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "Patient not found",
				Code:  "PATIENT_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Start transaction to handle related data
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete related observations first (cascade delete)
	if err := tx.Where("subject->>'reference' = ?", "Patient/"+id).Delete(&models.Observation{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete related observations",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	// Delete the patient
	if err := tx.Delete(&patient).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete patient",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to commit transaction",
			Message: err.Error(),
			Code:    "DATABASE_ERROR",
		})
		return
	}

	c.Status(http.StatusNoContent)
}
