package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/pkg/encryption"
)

// TestDatabaseURLEnv names the environment variable that enables the
// integration tests. Each test runs in a throwaway schema, which is dropped
// afterwards, so any database the URL can create schemas in will do.
const TestDatabaseURLEnv = "HEALTHHUB_TEST_DATABASE_URL"

// testDB returns a pool whose search_path points at a fresh, migrated
// schema. The test is skipped when no database is configured.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(TestDatabaseURLEnv)
	if url == "" {
		t.Skipf("set %s to run database integration tests", TestDatabaseURLEnv)
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		scanTimesInUTC(conn)
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func testKeys(t *testing.T) *encryption.Keys {
	t.Helper()
	k, err := encryption.NewKeys(bytes.Repeat([]byte{7}, encryption.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func ptr[T any](v T) *T { return &v }

// ticker returns a clock that advances one second per call, so rows get
// distinct, ordered timestamps regardless of the platform clock resolution.
func ticker() func() time.Time {
	t := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time {
		t = t.Add(time.Second)
		return t
	}
}

func createUser(t *testing.T, users *Users, email string, roles ...string) *models.User {
	t.Helper()
	u := &models.User{Email: email, PasswordHash: "hash", FirstName: "Test", LastName: "User", Roles: roles, Active: true}
	if err := users.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func newPatient(family, given string) *models.Patient {
	bd := models.NewDate(1980, time.March, 14)
	return &models.Patient{
		Active:    true,
		Gender:    "female",
		Name:      []models.Name{{Use: "official", Family: family, Given: []string{given}}},
		BirthDate: &bd,
		Telecom: []models.Contact{
			{System: "phone", Value: "+1 (555) 010-0200", Use: "mobile"},
			{System: "email", Value: strings.ToLower(given) + "@example.org", Use: "home"},
		},
		Address: []models.Address{{Use: "home", Line: []string{"1 Main Street"}, City: "Springfield"}},
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testDB(t)
	applied, err := Migrate(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Errorf("second run applied %v", applied)
	}
}

func TestUsers(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	users := NewUsers(pool)

	admin := createUser(t, users, "admin@example.org", "admin")
	nurse := createUser(t, users, "nurse@example.org", "nurse")

	if err := users.Create(ctx, &models.User{Email: "nurse@example.org", PasswordHash: "x", FirstName: "a", LastName: "b"}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate email: err = %v, want ErrConflict", err)
	}
	if err := users.Create(ctx, &models.User{Email: "bad@example.org", PasswordHash: "x", FirstName: "a", LastName: "b", Roles: []string{"superuser"}}); err == nil {
		t.Error("unknown role accepted by the database")
	}

	got, err := users.GetByEmail(ctx, "nurse@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != nurse.ID || len(got.Roles) != 1 || got.Roles[0] != "nurse" {
		t.Errorf("GetByEmail = %+v", got)
	}
	if _, err := users.Get(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user: err = %v", err)
	}

	id := uuid.MustParse(nurse.ID)
	updated, err := users.Update(ctx, id, UserUpdate{Roles: &[]string{"practitioner", "nurse"}})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Active || len(updated.Roles) != 2 {
		t.Errorf("Update roles = %+v", updated)
	}
	updated, err = users.Update(ctx, id, UserUpdate{Active: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Active || len(updated.Roles) != 2 {
		t.Errorf("Update active = %+v", updated)
	}

	if err := users.SetPassword(ctx, id, "new-hash"); err != nil {
		t.Fatal(err)
	}
	if err := users.RecordLogin(ctx, uuid.MustParse(admin.ID)); err != nil {
		t.Fatal(err)
	}
	if got, _ := users.Get(ctx, uuid.MustParse(admin.ID)); got.LastLoginAt == nil {
		t.Error("last login not recorded")
	}

	n, err := users.CountAdmins(ctx)
	if err != nil || n != 1 {
		t.Errorf("CountAdmins = %d, %v", n, err)
	}
	list, total, err := users.List(ctx, Page{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 1 || list[0].Email != "admin@example.org" {
		t.Errorf("List = %d %+v", total, list)
	}
}

func TestPatientsAreEncryptedAtRest(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	patients := NewPatients(pool, testKeys(t))

	p := newPatient("Okonkwo", "Adaeze")
	if err := patients.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	var raw []byte
	if err := pool.QueryRow(ctx, "SELECT resource FROM patients WHERE id = $1", p.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Okonkwo", "Adaeze", "1980-03-14", "555", "Springfield"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Errorf("stored resource contains %q in plain text", secret)
		}
	}

	// A ciphertext moved to another row must not decrypt.
	other := newPatient("Other", "Person")
	if err := patients.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE patients SET resource = $1 WHERE id = $2", raw, other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := patients.Get(ctx, uuid.MustParse(other.ID)); !errors.Is(err, encryption.ErrDecrypt) {
		t.Errorf("swapped ciphertext: err = %v, want ErrDecrypt", err)
	}
}

func TestPatientsCRUDAndSearch(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	users := NewUsers(pool)
	patients := NewPatients(pool, testKeys(t))
	patients.now = ticker()
	author := createUser(t, users, "doc@example.org", "practitioner")

	ada := newPatient("Van der Berg", "Ada")
	ada.CreatedBy = author.ID
	if err := patients.Create(ctx, ada); err != nil {
		t.Fatal(err)
	}
	bob := newPatient("Smith", "Bob")
	bob.Gender = "male"
	bob.BirthDate = ptr(models.NewDate(1975, time.July, 1))
	if err := patients.Create(ctx, bob); err != nil {
		t.Fatal(err)
	}

	got, err := patients.Get(ctx, uuid.MustParse(ada.ID))
	if err != nil {
		t.Fatal(err)
	}
	if got.FullName() != "Ada Van der Berg" || got.BirthDate.String() != "1980-03-14" ||
		got.CreatedBy != author.ID || len(got.Address) != 1 {
		t.Errorf("Get = %+v", got)
	}

	tests := []struct {
		name   string
		filter PatientFilter
		want   []string
	}{
		{"all", PatientFilter{}, []string{bob.ID, ada.ID}},
		{"family ignores case and spacing", PatientFilter{Family: "  van DER berg"}, []string{ada.ID}},
		{"family is exact", PatientFilter{Family: "Berg"}, nil},
		{"birth date", PatientFilter{BirthDate: ptr(models.NewDate(1975, time.July, 1))}, []string{bob.ID}},
		{"phone in another format", PatientFilter{Telecom: "+15550100200"}, []string{bob.ID, ada.ID}},
		{"email ignores case", PatientFilter{Telecom: "ADA@example.org"}, []string{ada.ID}},
		{"gender", PatientFilter{Gender: "male"}, []string{bob.ID}},
		{"inactive", PatientFilter{Active: ptr(false)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, total, err := patients.List(ctx, tt.filter, Page{Limit: 10})
			if err != nil {
				t.Fatal(err)
			}
			if total != len(tt.want) || len(list) != len(tt.want) {
				t.Fatalf("got %d (total %d), want %d", len(list), total, len(tt.want))
			}
			for i, id := range tt.want {
				if list[i].ID != id {
					t.Errorf("result %d = %s, want %s", i, list[i].ID, id)
				}
			}
		})
	}

	page, total, err := patients.List(ctx, PatientFilter{}, Page{Limit: 1, Offset: 1})
	if err != nil || total != 2 || len(page) != 1 || page[0].ID != ada.ID {
		t.Errorf("second page = %v %d %v", page, total, err)
	}

	ada.Name[0].Family = "Okafor"
	ada.Active = false
	if err := patients.Update(ctx, ada); err != nil {
		t.Fatal(err)
	}
	if ada.CreatedBy != author.ID {
		t.Errorf("Update lost createdBy: %q", ada.CreatedBy)
	}
	if list, _, _ := patients.List(ctx, PatientFilter{Family: "okafor", Active: ptr(false)}, Page{Limit: 10}); len(list) != 1 {
		t.Errorf("search after update found %d", len(list))
	}
	if list, _, _ := patients.List(ctx, PatientFilter{Family: "Van der Berg"}, Page{Limit: 10}); len(list) != 0 {
		t.Errorf("stale index still matches %d", len(list))
	}

	missing := newPatient("x", "y")
	missing.ID = uuid.NewString()
	if err := patients.Update(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing: err = %v", err)
	}
	if err := patients.Delete(ctx, uuid.MustParse(bob.ID)); err != nil {
		t.Fatal(err)
	}
	if err := patients.Delete(ctx, uuid.MustParse(bob.ID)); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v", err)
	}
}

func newObservation(patientID, code string, at time.Time) *models.Observation {
	return &models.Observation{
		Status:   "final",
		Category: []models.Category{{Coding: []models.Coding{{System: "http://terminology.hl7.org/CodeSystem/observation-category", Code: "laboratory"}}}},
		Code: models.CodeableConcept{
			Coding: []models.Coding{{System: "http://loinc.org", Code: code, Display: "Glucose"}},
		},
		Subject:           models.Reference{Reference: "Patient/" + patientID},
		EffectiveDateTime: at,
		ValueQuantity:     &models.Quantity{Value: 5.4, Unit: "mmol/L"},
	}
}

func TestObservations(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	keys := testKeys(t)
	patients := NewPatients(pool, keys)
	observations := NewObservations(pool, keys)

	p := newPatient("Doe", "Jane")
	if err := patients.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	other := newPatient("Roe", "Rich")
	if err := patients.Create(ctx, other); err != nil {
		t.Fatal(err)
	}

	if err := observations.Create(ctx, newObservation(uuid.NewString(), "2339-0", time.Now())); !errors.Is(err, ErrInvalidReference) {
		t.Errorf("unknown patient: err = %v, want ErrInvalidReference", err)
	}
	bad := newObservation(p.ID, "2339-0", time.Now())
	bad.Subject.Reference = "Practitioner/" + p.ID
	if err := observations.Create(ctx, bad); !errors.Is(err, ErrInvalidReference) {
		t.Errorf("non-patient subject: err = %v", err)
	}

	day := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	o1 := newObservation(p.ID, "2339-0", day)
	o2 := newObservation(p.ID, "718-7", day.Add(24*time.Hour))
	o2.Category[0].Coding[0].Code = "vital-signs"
	o3 := newObservation(other.ID, "2339-0", day.Add(48*time.Hour))
	for _, o := range []*models.Observation{o1, o2, o3} {
		if err := observations.Create(ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	var raw []byte
	if err := pool.QueryRow(ctx, "SELECT resource FROM observations WHERE id = $1", o1.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("mmol")) || bytes.Contains(raw, []byte("2339-0")) {
		t.Error("observation stored in plain text")
	}

	got, err := observations.Get(ctx, uuid.MustParse(o1.ID))
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject.Reference != "Patient/"+p.ID || got.ValueQuantity == nil || got.ValueQuantity.Value != 5.4 {
		t.Errorf("Get = %+v", got)
	}

	pid := uuid.MustParse(p.ID)
	from, to := day.Add(12*time.Hour), day.Add(36*time.Hour)
	tests := []struct {
		name   string
		filter ObservationFilter
		want   []string
	}{
		{"all, newest first", ObservationFilter{}, []string{o3.ID, o2.ID, o1.ID}},
		{"patient", ObservationFilter{PatientID: &pid}, []string{o2.ID, o1.ID}},
		{"bare code", ObservationFilter{Code: "2339-0"}, []string{o3.ID, o1.ID}},
		{"system and code", ObservationFilter{Code: "http://loinc.org|718-7"}, []string{o2.ID}},
		{"category", ObservationFilter{Category: "Vital-Signs"}, []string{o2.ID}},
		{"date range", ObservationFilter{From: &from, To: &to}, []string{o2.ID}},
		{"status", ObservationFilter{Status: "preliminary"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, total, err := observations.List(ctx, tt.filter, Page{Limit: 10})
			if err != nil {
				t.Fatal(err)
			}
			if total != len(tt.want) || len(list) != len(tt.want) {
				t.Fatalf("got %d (total %d), want %d", len(list), total, len(tt.want))
			}
			for i, id := range tt.want {
				if list[i].ID != id {
					t.Errorf("result %d = %s, want %s", i, list[i].ID, id)
				}
			}
		})
	}

	o1.Status = "amended"
	o1.Subject.Reference = "Patient/" + other.ID
	if err := observations.Update(ctx, o1); err != nil {
		t.Fatal(err)
	}
	if got, _ := observations.Get(ctx, uuid.MustParse(o1.ID)); got.Status != "amended" || got.Subject.Reference != "Patient/"+other.ID {
		t.Errorf("after update = %+v", got)
	}

	// Deleting a patient removes the patient's observations.
	if err := patients.Delete(ctx, uuid.MustParse(other.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := observations.Get(ctx, uuid.MustParse(o3.ID)); !errors.Is(err, ErrNotFound) {
		t.Errorf("observation survived patient delete: %v", err)
	}
	if err := observations.Delete(ctx, uuid.MustParse(o2.ID)); err != nil {
		t.Fatal(err)
	}
	if err := observations.Delete(ctx, uuid.MustParse(o2.ID)); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v", err)
	}
}

func TestMigrateMovesLegacyTablesAside(t *testing.T) {
	url := os.Getenv(TestDatabaseURLEnv)
	if url == "" {
		t.Skipf("set %s to run database integration tests", TestDatabaseURLEnv)
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, fmt.Sprintf(
		"CREATE SCHEMA %s; CREATE TABLE %s.users (id text PRIMARY KEY, email text, password text)", schema, schema)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE; DROP TABLE IF EXISTS legacy.users", schema))
	}()

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		scanTimesInUTC(conn)
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var moved, created bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('legacy.users') IS NOT NULL, to_regclass($1) IS NOT NULL",
		schema+".users").Scan(&moved, &created); err != nil {
		t.Fatal(err)
	}
	if !moved || !created {
		t.Errorf("legacy table moved = %v, new table created = %v", moved, created)
	}
}
