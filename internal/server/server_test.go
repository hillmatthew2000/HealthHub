package server_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/problem"
)

// TestAccessPolicy checks every protected route against the expected role
// matrix. It is written out by hand rather than derived from auth.Can so a
// change to the policy has to be made deliberately in two places.
func TestAccessPolicy(t *testing.T) {
	e := newEnv(t)
	patient := createPatient(e, e.tokenFor(auth.RoleAdmin))
	obs := createObservation(e, e.tokenFor(auth.RoleAdmin), patient.ID)
	pPath, oPath := "/api/v1/patients/"+patient.ID, "/api/v1/observations/"+obs.ID

	type route struct {
		method, path string
		body         func() any
	}
	var (
		createPatientR = route{http.MethodPost, "/api/v1/patients", func() any { return patientBody() }}
		listPatients   = route{http.MethodGet, "/api/v1/patients", nil}
		readPatient    = route{http.MethodGet, pPath, nil}
		updatePatient  = route{http.MethodPut, pPath, func() any { return patientBody() }}
		deletePatient  = route{http.MethodDelete, "/api/v1/patients/" + uuid.NewString(), nil}
		patientObs     = route{http.MethodGet, pPath + "/observations", nil}
		createObs      = route{http.MethodPost, "/api/v1/observations", func() any { return observationBody(patient.ID) }}
		listObs        = route{http.MethodGet, "/api/v1/observations", nil}
		readObs        = route{http.MethodGet, oPath, nil}
		updateObs      = route{http.MethodPut, oPath, func() any { return observationBody(patient.ID) }}
		deleteObs      = route{http.MethodDelete, "/api/v1/observations/" + uuid.NewString(), nil}
		createUser     = route{http.MethodPost, "/api/v1/users", func() any { return newUserBody(uuid.NewString() + "@example.org") }}
		listUsers      = route{http.MethodGet, "/api/v1/users", nil}
		readUser       = route{http.MethodGet, "/api/v1/users/" + uuid.NewString(), nil}
		updateUser     = route{http.MethodPatch, "/api/v1/users/" + uuid.NewString(), func() any { return map[string]any{"active": true} }}
		auditEvents    = route{http.MethodGet, "/api/v1/audit-events", nil}
	)
	all := []route{createPatientR, listPatients, readPatient, updatePatient, deletePatient, patientObs,
		createObs, listObs, readObs, updateObs, deleteObs, createUser, listUsers, readUser, updateUser, auditEvents}

	allowed := map[auth.Role][]route{
		auth.RoleAdmin:        all,
		auth.RolePractitioner: {createPatientR, listPatients, readPatient, updatePatient, patientObs, createObs, listObs, readObs, updateObs},
		auth.RoleNurse:        {listPatients, readPatient, patientObs, listObs, readObs},
		auth.RoleLabTech:      {listPatients, readPatient, patientObs, createObs, listObs, readObs, updateObs},
		"":                    nil, // an account without roles
	}

	for role, ok := range allowed {
		var tok string
		if role == "" {
			tok = e.tokenFor()
		} else {
			tok = e.tokenFor(role)
		}
		permitted := map[string]bool{}
		for _, r := range ok {
			permitted[r.method+" "+r.path] = true
		}
		for _, r := range all {
			var body any
			if r.body != nil {
				body = r.body()
			}
			w := e.do(r.method, r.path, tok, body)
			if permitted[r.method+" "+r.path] {
				if w.Code == http.StatusForbidden || w.Code == http.StatusUnauthorized {
					t.Errorf("role %q: %s %s = %d, want access", role, r.method, r.path, w.Code)
				}
			} else if w.Code != http.StatusForbidden {
				t.Errorf("role %q: %s %s = %d, want 403", role, r.method, r.path, w.Code)
			} else {
				p := wantProblem(t, w, http.StatusForbidden, problem.CodeForbidden)
				if p.Detail != problem.Forbidden().Detail {
					t.Errorf("forbidden detail %q reveals more than necessary", p.Detail)
				}
			}
		}
	}
}

// TestForbiddenRequestsAreNotProcessed makes sure authorisation runs before
// the handler: a forbidden request must neither change data nor be audited.
func TestForbiddenRequestsAreNotProcessed(t *testing.T) {
	e := newEnv(t)
	patient := createPatient(e, e.tokenFor(auth.RolePractitioner))
	before := e.audit.count()

	nurse := e.tokenFor(auth.RoleNurse)
	wantProblem(t, e.do(http.MethodDelete, "/api/v1/patients/"+patient.ID, nurse, nil), http.StatusForbidden, problem.CodeForbidden)
	wantProblem(t, e.do(http.MethodPost, "/api/v1/patients", nurse, "not even JSON"), http.StatusForbidden, problem.CodeForbidden)

	if _, err := e.patients.Get(t.Context(), mustUUID(t, patient.ID)); err != nil {
		t.Errorf("patient was deleted by a forbidden request: %v", err)
	}
	if e.audit.count() != before {
		t.Error("a forbidden request was audited as PHI access")
	}
}

func TestRoutingProblems(t *testing.T) {
	e := newEnv(t)
	wantProblem(t, e.do(http.MethodGet, "/api/v2/patients", "", nil), http.StatusNotFound, problem.CodeNotFound)
	w := e.do(http.MethodPatch, "/api/v1/auth/login", "", nil)
	wantProblem(t, w, http.StatusMethodNotAllowed, problem.CodeMethodNotAllowed)
}

func TestHealth(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/health/live", "/health/ready", "/health"} {
		w := e.do(http.MethodGet, path, "", nil)
		wantStatus(t, w, http.StatusOK)
	}

	down := newEnv(t, withDBDown())
	wantStatus(t, down.do(http.MethodGet, "/health/live", "", nil), http.StatusOK)
	for _, path := range []string{"/health/ready", "/health"} {
		w := down.do(http.MethodGet, path, "", nil)
		wantStatus(t, w, http.StatusServiceUnavailable)
		h := decode[handlers.HealthStatus](t, w)
		if h.Checks["database"] != "down" {
			t.Errorf("%s: checks = %v", path, h.Checks)
		}
	}
}
