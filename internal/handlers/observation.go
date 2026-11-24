package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
	"github.com/hillmatthew2000/HealthHub/internal/store"
)

// ObservationStore is the persistence the observation handlers need.
type ObservationStore interface {
	Create(ctx context.Context, o *models.Observation) error
	Get(ctx context.Context, id uuid.UUID) (*models.Observation, error)
	List(ctx context.Context, f store.ObservationFilter, page store.Page) ([]models.Observation, int, error)
	Update(ctx context.Context, o *models.Observation) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PatientLookup reports whether a patient exists.
type PatientLookup interface {
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// ObservationHandler serves the observation endpoints.
type ObservationHandler struct {
	observations ObservationStore
	patients     PatientLookup
	audit        Auditor
	paging       Paging
	validator    *validator.Validate
}

// NewObservationHandler returns an observation handler.
func NewObservationHandler(observations ObservationStore, patients PatientLookup, audit Auditor, paging Paging) *ObservationHandler {
	return &ObservationHandler{
		observations: observations,
		patients:     patients,
		audit:        audit,
		paging:       paging,
		validator:    newValidator(),
	}
}

var observationStatuses = []string{
	"registered", "preliminary", "final", "amended", "corrected", "cancelled", "entered-in-error", "unknown",
}

// bindObservation decodes and validates an observation body, including the
// rules the struct tags cannot express.
func (h *ObservationHandler) bindObservation(c *gin.Context) (*models.Observation, bool) {
	var o models.Observation
	if !bindJSON(c, h.validator, &o) {
		return nil, false
	}
	var errs []problem.FieldError
	if !hasCode(o.Code) {
		errs = append(errs, problem.FieldError{Field: "code", Message: "must contain a coding with a code, or text"})
	}
	if id, ok := o.Subject.PatientID(); !ok || uuid.Validate(id) != nil {
		errs = append(errs, problem.FieldError{Field: "subject.reference", Message: "must be Patient/<patient id>"})
	}
	if len(errs) > 0 {
		problem.Write(c, problem.Validation(errs...))
		return nil, false
	}
	o.ID, o.CreatedBy = "", ""
	return &o, true
}

func hasCode(cc models.CodeableConcept) bool {
	if cc.Text != "" {
		return true
	}
	for _, c := range cc.Coding {
		if c.Code != "" {
			return true
		}
	}
	return false
}

// writeObservationError maps an observation store error, reporting a missing
// subject patient as a validation failure.
func writeObservationError(c *gin.Context, op string, err error) {
	if errors.Is(err, store.ErrInvalidReference) {
		problem.Write(c, problem.Validation(problem.FieldError{
			Field: "subject.reference", Message: "must reference an existing patient",
		}))
		return
	}
	storeError(c, op, "Observation", err)
}

// CreateObservation handles POST /observations.
func (h *ObservationHandler) CreateObservation(c *gin.Context) {
	o, ok := h.bindObservation(c)
	if !ok {
		return
	}
	if principal, ok := auth.PrincipalFrom(c); ok {
		o.CreatedBy = principal.UserID
	}
	if err := h.observations.Create(c.Request.Context(), o); err != nil {
		writeObservationError(c, "create observation", err)
		return
	}
	audit(c, h.audit, ActionCreate, "observation", o.ID)
	c.Header("Location", c.FullPath()+"/"+o.ID)
	c.JSON(http.StatusCreated, o)
}

// filter parses the shared observation list filters.
func (h *ObservationHandler) filter(c *gin.Context, errs *queryErrors) store.ObservationFilter {
	f := store.ObservationFilter{
		Status:   queryEnum(c, "status", errs, observationStatuses...),
		Category: c.Query("category"),
		Code:     c.Query("code"),
		From:     queryTime(c, "from", false, errs),
		To:       queryTime(c, "to", true, errs),
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		errs.add("from", "must not be after to")
	}
	return f
}

func (h *ObservationHandler) list(c *gin.Context, f store.ObservationFilter, page store.Page, pageNo, limit int) {
	observations, total, err := h.observations.List(c.Request.Context(), f, page)
	if err != nil {
		internalError(c, "list observations", err)
		return
	}
	ids := make([]string, len(observations))
	for i := range observations {
		ids[i] = observations[i].ID
	}
	if !audit(c, h.audit, ActionList, "observation", ids...) {
		return
	}
	c.JSON(http.StatusOK, newPage(observations, pageNo, limit, total))
}

// GetObservations handles GET /observations.
func (h *ObservationHandler) GetObservations(c *gin.Context) {
	var errs queryErrors
	page, pageNo, limit := h.paging.page(c, &errs)
	f := h.filter(c, &errs)
	f.PatientID = queryUUID(c, "patient", &errs)
	if errs.write(c) {
		return
	}
	h.list(c, f, page, pageNo, limit)
}

// GetPatientObservations handles GET /patients/:id/observations.
func (h *ObservationHandler) GetPatientObservations(c *gin.Context) {
	id, ok := pathID(c, "id", "Patient")
	if !ok {
		return
	}
	var errs queryErrors
	page, pageNo, limit := h.paging.page(c, &errs)
	f := h.filter(c, &errs)
	if errs.write(c) {
		return
	}
	exists, err := h.patients.Exists(c.Request.Context(), id)
	if err != nil {
		internalError(c, "check patient", err)
		return
	}
	if !exists {
		problem.Write(c, problem.NotFound("Patient not found."))
		return
	}
	f.PatientID = &id
	h.list(c, f, page, pageNo, limit)
}

// GetObservation handles GET /observations/:id.
func (h *ObservationHandler) GetObservation(c *gin.Context) {
	id, ok := pathID(c, "id", "Observation")
	if !ok {
		return
	}
	o, err := h.observations.Get(c.Request.Context(), id)
	if err != nil {
		storeError(c, "get observation", "Observation", err)
		return
	}
	if !audit(c, h.audit, ActionRead, "observation", o.ID) {
		return
	}
	c.JSON(http.StatusOK, o)
}

// UpdateObservation handles PUT /observations/:id, replacing the resource.
func (h *ObservationHandler) UpdateObservation(c *gin.Context) {
	id, ok := pathID(c, "id", "Observation")
	if !ok {
		return
	}
	o, ok := h.bindObservation(c)
	if !ok {
		return
	}
	o.ID = id.String()
	if err := h.observations.Update(c.Request.Context(), o); err != nil {
		writeObservationError(c, "update observation", err)
		return
	}
	audit(c, h.audit, ActionUpdate, "observation", o.ID)
	c.JSON(http.StatusOK, o)
}

// DeleteObservation handles DELETE /observations/:id.
func (h *ObservationHandler) DeleteObservation(c *gin.Context) {
	id, ok := pathID(c, "id", "Observation")
	if !ok {
		return
	}
	if err := h.observations.Delete(c.Request.Context(), id); err != nil {
		storeError(c, "delete observation", "Observation", err)
		return
	}
	audit(c, h.audit, ActionDelete, "observation", id.String())
	c.Status(http.StatusNoContent)
}
