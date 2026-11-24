-- Earlier releases created tables with GORM AutoMigrate, which never ran to
-- completion because it could not map the patient JSON fields. Anything it
-- left behind is moved to the "legacy" schema rather than dropped.
DO $$
DECLARE
    t text;
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'users'
          AND column_name = 'password'
    ) THEN
        CREATE SCHEMA IF NOT EXISTS legacy;
        FOREACH t IN ARRAY ARRAY['role_permissions', 'user_roles', 'permissions', 'roles',
                                 'observations', 'patients', 'users'] LOOP
            IF to_regclass(format('%I.%I', current_schema(), t)) IS NOT NULL THEN
                EXECUTE format('ALTER TABLE %I.%I SET SCHEMA legacy', current_schema(), t);
            END IF;
        END LOOP;
    END IF;
END
$$;

-- Staff accounts. Roles are checked against the roles defined in
-- internal/auth/roles.go, which also holds the permission table.
CREATE TABLE users (
    id            uuid        PRIMARY KEY,
    email         text        NOT NULL CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    password_hash text        NOT NULL,
    first_name    text        NOT NULL,
    last_name     text        NOT NULL,
    roles         text[]      NOT NULL DEFAULT '{}'
        CONSTRAINT users_roles_valid
        CHECK (roles <@ ARRAY['admin', 'practitioner', 'nurse', 'lab-tech']::text[]),
    active        boolean     NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    created_by    uuid        REFERENCES users (id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX users_email_key ON users (email);

-- Patients. The FHIR resource is stored only as AES-256-GCM ciphertext in
-- "resource"; the *_bidx columns hold keyed HMAC blind indexes that allow
-- exact-match search without storing plaintext.
CREATE TABLE patients (
    id             uuid        PRIMARY KEY,
    active         boolean     NOT NULL DEFAULT true,
    gender         text        NOT NULL CHECK (gender IN ('male', 'female', 'other', 'unknown')),
    resource       bytea       NOT NULL,
    family_bidx    text[]      NOT NULL DEFAULT '{}',
    birthdate_bidx text,
    telecom_bidx   text[]      NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    created_by     uuid        REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX patients_created_at_idx ON patients (created_at DESC, id);
CREATE INDEX patients_family_bidx_idx ON patients USING gin (family_bidx);
CREATE INDEX patients_telecom_bidx_idx ON patients USING gin (telecom_bidx);
CREATE INDEX patients_birthdate_bidx_idx ON patients (birthdate_bidx);

-- Observations. Status, category codes and the effective time stay in clear
-- text for filtering; the observation codes are blind indexed and the full
-- resource, values included, is encrypted.
CREATE TABLE observations (
    id           uuid        PRIMARY KEY,
    patient_id   uuid        NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    status       text        NOT NULL CHECK (status IN ('registered', 'preliminary', 'final', 'amended',
                                                        'corrected', 'cancelled', 'entered-in-error', 'unknown')),
    category     text[]      NOT NULL DEFAULT '{}',
    code_bidx    text[]      NOT NULL DEFAULT '{}',
    effective_at timestamptz NOT NULL,
    resource     bytea       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    created_by   uuid        REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX observations_patient_effective_idx ON observations (patient_id, effective_at DESC, id);
CREATE INDEX observations_effective_idx ON observations (effective_at DESC, id);
CREATE INDEX observations_code_bidx_idx ON observations USING gin (code_bidx);
CREATE INDEX observations_category_idx ON observations USING gin (category);
