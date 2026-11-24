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

func newUserBody(email string, roles ...string) map[string]any {
	return map[string]any{
		"email": email, "password": testPassword, "firstName": "Sam", "lastName": "Lee", "roles": roles,
	}
}

func TestCreateUser(t *testing.T) {
	e := newEnv(t)
	admin := e.addUser("admin@example.org", auth.RoleAdmin)
	tok := e.token(admin)

	w := e.do(http.MethodPost, "/api/v1/users", tok, newUserBody("Sam.Lee@Example.org", "nurse", "practitioner", "nurse"))
	wantStatus(t, w, http.StatusCreated)
	info := decode[models.UserInfo](t, w)
	if info.Email != "sam.lee@example.org" || !info.Active {
		t.Errorf("user = %+v", info)
	}
	if strings.Join(info.Roles, ",") != "nurse,practitioner" {
		t.Errorf("roles = %v, want them de-duplicated and sorted", info.Roles)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Errorf("response exposes the password: %s", w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/api/v1/users/"+info.ID {
		t.Errorf("Location = %q", loc)
	}
	stored, _ := e.users.Get(t.Context(), mustUUID(t, info.ID))
	if stored.CreatedBy != admin.ID {
		t.Errorf("createdBy = %q, want %q", stored.CreatedBy, admin.ID)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionCreate) || ev.ResourceType != "user" || ev.ActorID != admin.ID {
		t.Errorf("audit event = %+v", ev)
	}

	// The new account can sign in with the password the administrator set.
	login(e, "sam.lee@example.org", testPassword)

	w = e.do(http.MethodPost, "/api/v1/users", tok, newUserBody("sam.lee@example.org"))
	wantProblem(t, w, http.StatusConflict, problem.CodeConflict)

	bad := newUserBody("x@example.org", "superuser")
	bad["password"] = "short"
	p := wantProblem(t, e.do(http.MethodPost, "/api/v1/users", tok, bad), http.StatusUnprocessableEntity, problem.CodeValidationFailed)
	wantFields(t, p, "password", "roles[0]")
}

func TestListAndGetUsers(t *testing.T) {
	e := newEnv(t)
	tok := e.token(e.addUser("admin@example.org", auth.RoleAdmin))
	nurse := e.addUser("nurse@example.org", auth.RoleNurse)

	w := e.do(http.MethodGet, "/api/v1/users?limit=1", tok, nil)
	wantStatus(t, w, http.StatusOK)
	page := decode[handlers.PageResponse[models.UserInfo]](t, w)
	if page.Total != 2 || len(page.Data) != 1 || page.Data[0].Email != "admin@example.org" {
		t.Errorf("page = %+v", page)
	}
	if strings.Contains(w.Body.String(), "passwordHash") {
		t.Error("user list exposes password hashes")
	}

	w = e.do(http.MethodGet, "/api/v1/users/"+nurse.ID, tok, nil)
	wantStatus(t, w, http.StatusOK)
	if got := decode[models.UserInfo](t, w); got.ID != nurse.ID {
		t.Errorf("user = %+v", got)
	}
	wantProblem(t, e.do(http.MethodGet, "/api/v1/users/"+uuid.NewString(), tok, nil), http.StatusNotFound, problem.CodeNotFound)
	wantProblem(t, e.do(http.MethodGet, "/api/v1/users/me", tok, nil), http.StatusBadRequest, problem.CodeInvalidRequest)
	wantProblem(t, e.do(http.MethodGet, "/api/v1/users?page=-1", tok, nil), http.StatusBadRequest, problem.CodeInvalidRequest)
}

func TestUpdateUser(t *testing.T) {
	e := newEnv(t)
	admin := e.addUser("admin@example.org", auth.RoleAdmin)
	tok := e.token(admin)
	nurse := e.addUser("nurse@example.org", auth.RoleNurse)
	path := "/api/v1/users/" + nurse.ID

	w := e.do(http.MethodPatch, path, tok, map[string]any{"roles": []string{"practitioner", "lab-tech"}})
	wantStatus(t, w, http.StatusOK)
	if got := decode[models.UserInfo](t, w); strings.Join(got.Roles, ",") != "lab-tech,practitioner" || !got.Active {
		t.Errorf("user = %+v", got)
	}
	if ev, _ := e.audit.last(); ev.Action != string(handlers.ActionUpdate) || ev.ResourceIDs[0] != nurse.ID {
		t.Errorf("audit event = %+v", ev)
	}

	w = e.do(http.MethodPatch, path, tok, map[string]any{"active": false})
	wantStatus(t, w, http.StatusOK)
	if got := decode[models.UserInfo](t, w); got.Active || len(got.Roles) != 2 {
		t.Errorf("an update of active changed the roles: %+v", got)
	}
	// The deactivated account can no longer sign in.
	wantProblem(t, e.do(http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"email": nurse.Email, "password": testPassword}), http.StatusUnauthorized, problem.CodeInvalidCredentials)

	p := wantProblem(t, e.do(http.MethodPatch, path, tok, map[string]any{}), http.StatusUnprocessableEntity, problem.CodeValidationFailed)
	wantFields(t, p, "body")
	p = wantProblem(t, e.do(http.MethodPatch, path, tok, map[string]any{"roles": []string{"root"}}),
		http.StatusUnprocessableEntity, problem.CodeValidationFailed)
	wantFields(t, p, "roles[0]")
	wantProblem(t, e.do(http.MethodPatch, "/api/v1/users/"+uuid.NewString(), tok, map[string]any{"active": true}),
		http.StatusNotFound, problem.CodeNotFound)
}

func TestLastAdministratorIsKept(t *testing.T) {
	e := newEnv(t)
	admin := e.addUser("admin@example.org", auth.RoleAdmin)
	tok := e.token(admin)
	self := "/api/v1/users/" + admin.ID

	for _, body := range []map[string]any{{"roles": []string{"nurse"}}, {"active": false}, {"roles": []string{}}} {
		wantProblem(t, e.do(http.MethodPatch, self, tok, body), http.StatusConflict, problem.CodeConflict)
	}
	if got, _ := e.users.Get(t.Context(), mustUUID(t, admin.ID)); !got.Active || len(got.Roles) != 1 {
		t.Errorf("rejected update changed the account: %+v", got)
	}

	// With a second administrator the first may step down.
	second := e.addUser("second@example.org", auth.RoleAdmin)
	wantStatus(t, e.do(http.MethodPatch, self, tok, map[string]any{"roles": []string{"practitioner"}}), http.StatusOK)
	wantProblem(t, e.do(http.MethodPatch, "/api/v1/users/"+second.ID, e.token(second), map[string]any{"active": false}),
		http.StatusConflict, problem.CodeConflict)
}

func TestAuditTrail(t *testing.T) {
	e := newEnv(t)
	admin := e.addUser("admin@example.org", auth.RoleAdmin)
	doc := e.addUser("doc@example.org", auth.RolePractitioner)
	p := createPatient(e, e.token(doc))
	wantStatus(t, e.do(http.MethodGet, "/api/v1/patients/"+p.ID, e.token(doc), nil), http.StatusOK)

	w := e.do(http.MethodGet, "/api/v1/audit-events?actor="+doc.ID+"&resourceType=patient", e.token(admin), nil)
	wantStatus(t, w, http.StatusOK)
	page := decode[handlers.PageResponse[models.AuditEvent]](t, w)
	if page.Total != 2 || page.Data[0].Action != "read" || page.Data[1].Action != "create" {
		t.Errorf("audit events = %+v", page.Data)
	}

	wantProblem(t, e.do(http.MethodGet, "/api/v1/audit-events", e.token(doc), nil), http.StatusForbidden, problem.CodeForbidden)
	bad := wantProblem(t, e.do(http.MethodGet, "/api/v1/audit-events?actor=x&resourceType=car&from=2026-02-01&to=2026-01-01", e.token(admin), nil),
		http.StatusBadRequest, problem.CodeInvalidRequest)
	wantFields(t, bad, "actor", "resourceType", "from")
}

// TestReadsFailClosedWithoutAudit checks that PHI is not disclosed when its
// disclosure cannot be recorded, while writes, which have already happened,
// still report success.
func TestReadsFailClosedWithoutAudit(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenFor(auth.RolePractitioner)
	p := createPatient(e, tok)
	e.audit.err = errBoom

	for _, path := range []string{"/api/v1/patients/" + p.ID, "/api/v1/patients", "/api/v1/patients/" + p.ID + "/observations"} {
		w := e.do(http.MethodGet, path, tok, nil)
		wantProblem(t, w, http.StatusInternalServerError, problem.CodeInternal)
		if strings.Contains(w.Body.String(), "Rivera") {
			t.Errorf("%s disclosed PHI without an audit record", path)
		}
	}
	wantStatus(t, e.do(http.MethodPost, "/api/v1/patients", tok, patientBody()), http.StatusCreated)
}
