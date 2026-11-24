package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/store"
)

// PatientStore is the persistence the patient handlers need.
type PatientStore interface {
	Create(ctx context.Context, p *models.Patient) error
	Get(ctx context.Context, id uuid.UUID) (*models.Patient, error)
	List(ctx context.Context, f store.PatientFilter, page store.Page) ([]models.Patient, int, error)
	Update(ctx context.Context, p *models.Patient) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PatientHandler serves the patient endpoints.
type PatientHandler struct {
	patients  PatientStore
	audit     Auditor
	paging    Paging
	validator *validator.Validate
}

// NewPatientHandler returns a patient handler.
func NewPatientHandler(patients PatientStore, audit Auditor, paging Paging) *PatientHandler {
	return &PatientHandler{patients: patients, audit: audit, paging: paging, validator: newValidator()}
}

// bindPatient decodes a patient body. Server-managed fields in the body are
// ignored, and a missing "active" means true.
func (h *PatientHandler) bindPatient(c *gin.Context) (*models.Patient, bool) {
	p := models.Patient{Active: true}
	if !bindJSON(c, h.validator, &p) {
		return nil, false
	}
	p.ID, p.CreatedBy = "", ""
	return &p, true
}

// CreatePatient handles POST /patients.
func (h *PatientHandler) CreatePatient(c *gin.Context) {
	p, ok := h.bindPatient(c)
	if !ok {
		return
	}
	if principal, ok := auth.PrincipalFrom(c); ok {
		p.CreatedBy = principal.UserID
	}
	if err := h.patients.Create(c.Request.Context(), p); err != nil {
		storeError(c, "create patient", "Patient", err)
		return
	}
	h.audit.Record(c, ActionCreate, "patient", p.ID)
	c.Header("Location", c.FullPath()+"/"+p.ID)
	c.JSON(http.StatusCreated, p)
}

// GetPatients handles GET /patients. Filters are exact matches because the
// underlying values are stored encrypted.
func (h *PatientHandler) GetPatients(c *gin.Context) {
	var errs queryErrors
	if _, ok := c.GetQuery("search"); ok {
		errs.add("search", "free-text search is not supported; filter by family, birthdate or telecom")
	}
	page, pageNo, limit := h.paging.page(c, &errs)
	f := store.PatientFilter{
		Family:  c.Query("family"),
		Telecom: c.Query("telecom"),
		Gender:  queryEnum(c, "gender", &errs, "male", "female", "other", "unknown"),
		Active:  queryBool(c, "active", &errs),
	}
	if v, ok := c.GetQuery("birthdate"); ok {
		d, err := models.ParseDate(v)
		if err != nil {
			errs.add("birthdate", "must be a YYYY-MM-DD date")
		} else {
			f.BirthDate = &d
		}
	}
	if errs.write(c) {
		return
	}

	patients, total, err := h.patients.List(c.Request.Context(), f, page)
	if err != nil {
		internalError(c, "list patients", err)
		return
	}
	ids := make([]string, len(patients))
	for i := range patients {
		ids[i] = patients[i].ID
	}
	h.audit.Record(c, ActionList, "patient", ids...)
	c.JSON(http.StatusOK, newPage(patients, pageNo, limit, total))
}

// GetPatient handles GET /patients/:id.
func (h *PatientHandler) GetPatient(c *gin.Context) {
	id, ok := pathID(c, "id", "Patient")
	if !ok {
		return
	}
	p, err := h.patients.Get(c.Request.Context(), id)
	if err != nil {
		storeError(c, "get patient", "Patient", err)
		return
	}
	h.audit.Record(c, ActionRead, "patient", p.ID)
	c.JSON(http.StatusOK, p)
}

// UpdatePatient handles PUT /patients/:id, replacing the resource.
func (h *PatientHandler) UpdatePatient(c *gin.Context) {
	id, ok := pathID(c, "id", "Patient")
	if !ok {
		return
	}
	p, ok := h.bindPatient(c)
	if !ok {
		return
	}
	p.ID = id.String()
	if err := h.patients.Update(c.Request.Context(), p); err != nil {
		storeError(c, "update patient", "Patient", err)
		return
	}
	h.audit.Record(c, ActionUpdate, "patient", p.ID)
	c.JSON(http.StatusOK, p)
}

// DeletePatient handles DELETE /patients/:id. The patient's observations
// are deleted with it.
func (h *PatientHandler) DeletePatient(c *gin.Context) {
	id, ok := pathID(c, "id", "Patient")
	if !ok {
		return
	}
	if err := h.patients.Delete(c.Request.Context(), id); err != nil {
		storeError(c, "delete patient", "Patient", err)
		return
	}
	h.audit.Record(c, ActionDelete, "patient", id.String())
	c.Status(http.StatusNoContent)
}
