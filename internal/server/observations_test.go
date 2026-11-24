package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

func createObservation(e *env, token, patientID string) models.Observation {
	e.t.Helper()
	w := e.do(http.MethodPost, "/api/v1/observations", token, observationBody(patientID))
	wantStatus(e.t, w, http.StatusCreated)
	return decode[models.Observation](e.t, w)
}

func TestCreateObservation(t *testing.T) {
	e := newEnv(t)
	patient := createPatient(e, e.tokenFor(auth.RolePractitioner))
	lab := e.addUser("lab@example.org", auth.RoleLabTech)

	w := e.do(http.MethodPost, "/api/v1/observations", e.token(lab), observationBody(patient.ID))
	wantStatus(t, w, http.StatusCreated)
	o := decode[models.Observation](t, w)
	if uuid.Validate(o.ID) != nil || o.CreatedBy != lab.ID {
		t.Errorf("observation = %+v", o)
	}
	if o.ValueQuantity == nil || o.ValueQuantity.Value != 72 {
		t.Errorf("value = %+v", o.ValueQuantity)
	}
	if loc := w.Header().Get("Location"); loc != "/api/v1/observations/"+o.ID {
		t.Errorf("Location = %q", loc)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionCreate) || ev.ResourceType != "observation" {
		t.Errorf("audit event = %+v", ev)
	}
}

func TestCreateObservationValidation(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	patient := createPatient(e, tok)

	noCode := observationBody(patient.ID)
	noCode["code"] = map[string]any{"coding": []map[string]any{{"system": "http://loinc.org"}}}
	badSubject := observationBody(patient.ID)
	badSubject["subject"] = map[string]any{"reference": "Practitioner/" + patient.ID}
	badStatus := observationBody(patient.ID)
	badStatus["status"] = "done"
	delete(badStatus, "effectiveDateTime")

	tests := []struct {
		name   string
		body   any
		status int
		code   string
		fields []string
	}{
		{"malformed JSON", `{"status":`, http.StatusBadRequest, problem.CodeInvalidRequest, nil},
		{"bad timestamp", `{"status":"final","effectiveDateTime":"yesterday"}`, http.StatusBadRequest, problem.CodeInvalidRequest, nil},
		{"no code", noCode, http.StatusUnprocessableEntity, problem.CodeValidationFailed, []string{"code"}},
		{"subject is not a patient", badSubject, http.StatusUnprocessableEntity, problem.CodeValidationFailed, []string{"subject.reference"}},
		{"unknown patient", observationBody(uuid.NewString()), http.StatusUnprocessableEntity, problem.CodeValidationFailed, []string{"subject.reference"}},
		{"bad status and no time", badStatus, http.StatusUnprocessableEntity, problem.CodeValidationFailed, []string{"status", "effectiveDateTime"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(http.MethodPost, "/api/v1/observations", tok, tt.body)
			p := wantProblem(t, w, tt.status, tt.code)
			if tt.fields != nil {
				wantFields(t, p, tt.fields...)
			}
		})
	}
	if len(e.observations.byID) != 0 {
		t.Error("a rejected request created an observation")
	}
}

func TestGetUpdateDeleteObservation(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	patient := createPatient(e, tok)
	o := createObservation(e, tok, patient.ID)
	path := "/api/v1/observations/" + o.ID

	w := e.do(http.MethodGet, path, e.tokenFor(auth.RoleNurse), nil)
	wantStatus(t, w, http.StatusOK)
	if got := decode[models.Observation](t, w); got.ID != o.ID {
		t.Errorf("observation = %+v", got)
	}

	body := observationBody(patient.ID)
	body["status"] = "amended"
	w = e.do(http.MethodPut, path, tok, body)
	wantStatus(t, w, http.StatusOK)
	if got := decode[models.Observation](t, w); got.Status != "amended" || got.ID != o.ID {
		t.Errorf("updated observation = %+v", got)
	}

	// Moving an observation to a patient that does not exist is rejected.
	w = e.do(http.MethodPut, path, tok, observationBody(uuid.NewString()))
	wantProblem(t, w, http.StatusUnprocessableEntity, problem.CodeValidationFailed)

	wantProblem(t, e.do(http.MethodPut, "/api/v1/observations/"+uuid.NewString(), tok, observationBody(patient.ID)),
		http.StatusNotFound, problem.CodeNotFound)
	wantProblem(t, e.do(http.MethodGet, "/api/v1/observations/123", tok, nil),
		http.StatusBadRequest, problem.CodeInvalidRequest)

	admin := e.tokenFor(auth.RoleAdmin)
	wantStatus(t, e.do(http.MethodDelete, path, admin, nil), http.StatusNoContent)
	wantProblem(t, e.do(http.MethodGet, path, admin, nil), http.StatusNotFound, problem.CodeNotFound)
	wantProblem(t, e.do(http.MethodDelete, path, admin, nil), http.StatusNotFound, problem.CodeNotFound)
}

func TestListObservations(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	p1, p2 := createPatient(e, tok), createPatient(e, tok)
	createObservation(e, tok, p1.ID)
	createObservation(e, tok, p1.ID)
	createObservation(e, tok, p2.ID)

	w := e.do(http.MethodGet, "/api/v1/patients/"+p1.ID+"/observations?from=2026-01-01&to=2026-01-31&status=final&code=8867-4", tok, nil)
	wantStatus(t, w, http.StatusOK)
	page := decode[handlers.PageResponse[models.Observation]](t, w)
	if page.Total != 2 || len(page.Data) != 2 {
		t.Errorf("page = %+v", page)
	}
	f := e.observations.filters[len(e.observations.filters)-1]
	if f.PatientID == nil || f.PatientID.String() != p1.ID || f.Status != "final" || f.Code != "8867-4" {
		t.Errorf("filter = %+v", f)
	}
	// A date "to" bound covers the whole day.
	if f.From == nil || !f.From.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) ||
		f.To == nil || f.To.Before(time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)) {
		t.Errorf("time range = %v .. %v", f.From, f.To)
	}

	w = e.do(http.MethodGet, "/api/v1/observations?patient="+p2.ID, tok, nil)
	wantStatus(t, w, http.StatusOK)
	if page := decode[handlers.PageResponse[models.Observation]](t, w); page.Total != 1 {
		t.Errorf("observations for patient 2 = %+v", page)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionList) || len(ev.ResourceIDs) != 1 {
		t.Errorf("audit event = %+v", ev)
	}

	wantProblem(t, e.do(http.MethodGet, "/api/v1/patients/"+uuid.NewString()+"/observations", tok, nil),
		http.StatusNotFound, problem.CodeNotFound)
	wantProblem(t, e.do(http.MethodGet, "/api/v1/patients/nope/observations", tok, nil),
		http.StatusBadRequest, problem.CodeInvalidRequest)
}

func TestListObservationsRejectsBadQueries(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	tests := []struct {
		query  string
		fields []string
	}{
		{"status=done", []string{"status"}},
		{"from=2026-02-01&to=2026-01-01", []string{"from"}},
		{"from=last-week", []string{"from"}},
		{"patient=42", []string{"patient"}},
		{"limit=0", []string{"limit"}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			p := wantProblem(t, e.do(http.MethodGet, "/api/v1/observations?"+tt.query, tok, nil),
				http.StatusBadRequest, problem.CodeInvalidRequest)
			wantFields(t, p, tt.fields...)
		})
	}
}
