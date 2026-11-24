package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

// createPatient creates a patient through the API and returns it.
func createPatient(e *env, token string) models.Patient {
	e.t.Helper()
	w := e.do(http.MethodPost, "/api/v1/patients", token, patientBody())
	wantStatus(e.t, w, http.StatusCreated)
	return decode[models.Patient](e.t, w)
}

func TestCreatePatient(t *testing.T) {
	e := newEnv(t)
	doc := e.addUser("doc@example.org", auth.RolePractitioner)

	body := patientBody()
	body["id"] = "client-chosen"
	body["createdBy"] = "someone-else"
	w := e.do(http.MethodPost, "/api/v1/patients", e.token(doc), body)
	wantStatus(t, w, http.StatusCreated)
	p := decode[models.Patient](t, w)

	if uuid.Validate(p.ID) != nil {
		t.Errorf("server did not assign an ID: %q", p.ID)
	}
	if p.CreatedBy != doc.ID {
		t.Errorf("createdBy = %q, want the caller %q", p.CreatedBy, doc.ID)
	}
	if !p.Active {
		t.Error("a patient without \"active\" should default to active")
	}
	if p.BirthDate == nil || p.BirthDate.String() != "1984-03-02" {
		t.Errorf("birthDate = %v", p.BirthDate)
	}
	if loc := w.Header().Get("Location"); loc != "/api/v1/patients/"+p.ID {
		t.Errorf("Location = %q", loc)
	}
	ev, _ := e.audit.last()
	if ev.Action != string(handlers.ActionCreate) || ev.ResourceType != "patient" || ev.ActorID != doc.ID ||
		len(ev.ResourceIDs) != 1 || ev.ResourceIDs[0] != p.ID {
		t.Errorf("audit event = %+v", ev)
	}
}

func TestCreatePatientRejectsBadInput(t *testing.T) {
	e := newEnv(t, withMaxBody(4096))
	tok := e.tokenFor(auth.RolePractitioner)

	invalid := patientBody()
	invalid["gender"] = "robot"
	invalid["name"] = []map[string]any{{"family": "", "given": []string{}}}
	invalid["telecom"] = []map[string]any{{"system": "pigeon", "value": "x"}}

	tests := []struct {
		name   string
		body   any
		status int
		code   string
		fields []string
	}{
		{"malformed JSON", `{"name": [`, http.StatusBadRequest, problem.CodeInvalidRequest, nil},
		{"no body", "", http.StatusBadRequest, problem.CodeInvalidRequest, nil},
		{"wrong type", `{"name": "Ana Rivera"}`, http.StatusBadRequest, problem.CodeInvalidRequest, nil},
		{"invalid birth date", `{"name":[{"family":"R","given":["A"]}],"gender":"male","birthDate":"02/03/1984"}`,
			http.StatusBadRequest, problem.CodeInvalidRequest, nil},
		{"missing required fields", map[string]any{}, http.StatusUnprocessableEntity, problem.CodeValidationFailed,
			[]string{"name", "gender"}},
		{"invalid fields", invalid, http.StatusUnprocessableEntity, problem.CodeValidationFailed,
			[]string{"gender", "name[0].family", "name[0].given", "telecom[0].system"}},
		{"too large", `{"name":[{"family":"` + strings.Repeat("x", 5000) + `"}]}`,
			http.StatusRequestEntityTooLarge, problem.CodePayloadTooLarge, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := e.do(http.MethodPost, "/api/v1/patients", tok, tt.body)
			p := wantProblem(t, w, tt.status, tt.code)
			if tt.fields != nil {
				wantFields(t, p, tt.fields...)
			}
		})
	}
	if len(e.patients.byID) != 0 {
		t.Error("a rejected request created a patient")
	}
	if e.audit.count() != 0 {
		t.Error("a rejected request was audited as PHI access")
	}
}

func TestGetPatient(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RoleNurse)
	created := createPatient(e, e.tokenFor(auth.RolePractitioner))

	w := e.do(http.MethodGet, "/api/v1/patients/"+created.ID, tok, nil)
	wantStatus(t, w, http.StatusOK)
	if got := decode[models.Patient](t, w); got.ID != created.ID || got.FullName() != "Ana Rivera" {
		t.Errorf("patient = %+v", got)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionRead) || ev.ResourceIDs[0] != created.ID {
		t.Errorf("audit event = %+v", ev)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q; PHI responses must not be cached", cc)
	}

	w = e.do(http.MethodGet, "/api/v1/patients/not-a-uuid", tok, nil)
	wantProblem(t, w, http.StatusBadRequest, problem.CodeInvalidRequest)

	w = e.do(http.MethodGet, "/api/v1/patients/"+uuid.NewString(), tok, nil)
	wantProblem(t, w, http.StatusNotFound, problem.CodeNotFound)
}

func TestListPatients(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	for range 3 {
		createPatient(e, tok)
	}

	w := e.do(http.MethodGet, "/api/v1/patients?limit=2&page=2", tok, nil)
	wantStatus(t, w, http.StatusOK)
	page := decode[handlers.PageResponse[models.Patient]](t, w)
	if page.Page != 2 || page.Limit != 2 || page.Total != 3 || page.TotalPages != 2 || len(page.Data) != 1 {
		t.Errorf("page = %+v", page)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionList) || len(ev.ResourceIDs) != 1 || ev.ResourceIDs[0] != page.Data[0].ID {
		t.Errorf("audit event = %+v; only the returned records should be logged", ev)
	}

	// Filters reach the store parsed.
	w = e.do(http.MethodGet, "/api/v1/patients?family=Rivera&birthdate=1984-03-02&telecom=%2B15550100&gender=female&active=true", tok, nil)
	wantStatus(t, w, http.StatusOK)
	f := e.patients.filters[len(e.patients.filters)-1]
	if f.Family != "Rivera" || f.BirthDate == nil || f.BirthDate.String() != "1984-03-02" ||
		f.Telecom != "+15550100" || f.Gender != "female" || f.Active == nil || !*f.Active {
		t.Errorf("filter = %+v", f)
	}

	// An empty result is an empty array, not null.
	w = e.do(http.MethodGet, "/api/v1/patients?gender=male", tok, nil)
	wantStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestListPatientsRejectsBadQueries(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	tests := []struct {
		query  string
		fields []string
	}{
		{"search=rivera", []string{"search"}},
		{"birthdate=1984-13-45", []string{"birthdate"}},
		{"gender=robot&active=maybe", []string{"gender", "active"}},
		{"page=0&limit=1000", []string{"page", "limit"}},
		{"limit=abc", []string{"limit"}},
		{"page=999999999", []string{"page"}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			w := e.do(http.MethodGet, "/api/v1/patients?"+tt.query, tok, nil)
			p := wantProblem(t, w, http.StatusBadRequest, problem.CodeInvalidRequest)
			wantFields(t, p, tt.fields...)
		})
	}
	if len(e.patients.filters) != 0 {
		t.Error("an invalid query reached the store")
	}
}

func TestUpdatePatient(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	created := createPatient(e, tok)

	body := patientBody()
	body["gender"] = "other"
	body["active"] = false
	w := e.do(http.MethodPut, "/api/v1/patients/"+created.ID, tok, body)
	wantStatus(t, w, http.StatusOK)
	got := decode[models.Patient](t, w)
	if got.ID != created.ID || got.Gender != "other" || got.Active || got.CreatedBy != created.CreatedBy {
		t.Errorf("updated patient = %+v", got)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionUpdate) {
		t.Errorf("audit event = %+v", ev)
	}

	w = e.do(http.MethodPut, "/api/v1/patients/"+uuid.NewString(), tok, patientBody())
	wantProblem(t, w, http.StatusNotFound, problem.CodeNotFound)

	w = e.do(http.MethodPut, "/api/v1/patients/"+created.ID, tok, map[string]any{"gender": "female"})
	p := wantProblem(t, w, http.StatusUnprocessableEntity, problem.CodeValidationFailed)
	wantFields(t, p, "name")
}

func TestDeletePatient(t *testing.T) {
	e := newEnv(t)
	created := createPatient(e, e.tokenFor(auth.RolePractitioner))
	admin := e.tokenFor(auth.RoleAdmin)

	wantStatus(t, e.do(http.MethodDelete, "/api/v1/patients/"+created.ID, admin, nil), http.StatusNoContent)
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionDelete) || ev.ResourceIDs[0] != created.ID {
		t.Errorf("audit event = %+v", ev)
	}
	wantProblem(t, e.do(http.MethodDelete, "/api/v1/patients/"+created.ID, admin, nil),
		http.StatusNotFound, problem.CodeNotFound)
	wantProblem(t, e.do(http.MethodGet, "/api/v1/patients/"+created.ID, admin, nil),
		http.StatusNotFound, problem.CodeNotFound)
}

func TestPatientStoreFailure(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RoleAdmin)
	e.patients.err = errBoom
	for _, req := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/patients"},
		{http.MethodGet, "/api/v1/patients/" + uuid.NewString()},
		{http.MethodDelete, "/api/v1/patients/" + uuid.NewString()},
	} {
		w := e.do(req.method, req.path, tok, nil)
		wantProblem(t, w, http.StatusInternalServerError, problem.CodeInternal)
		if strings.Contains(w.Body.String(), "SELECT") {
			t.Errorf("%s %s leaked the store error: %s", req.method, req.path, w.Body.String())
		}
	}
}
