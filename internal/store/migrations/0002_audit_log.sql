-- Who accessed which protected health information, when and how. Rows
-- identify records by ID only and never contain PHI. The actor is kept
-- without a foreign key so the trail survives the deletion of an account,
-- and rows cannot be changed once written.
CREATE TABLE audit_log (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at   timestamptz NOT NULL DEFAULT now(),
    actor_id      uuid,
    action        text        NOT NULL CHECK (action IN ('create', 'read', 'list', 'update', 'delete')),
    resource_type text        NOT NULL CHECK (resource_type IN ('patient', 'observation', 'user')),
    resource_ids  uuid[]      NOT NULL DEFAULT '{}',
    request_id    text,
    client_ip     inet
);

CREATE INDEX audit_log_occurred_at_idx ON audit_log (occurred_at DESC, id);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX audit_log_resource_ids_idx ON audit_log USING gin (resource_ids);

CREATE FUNCTION audit_log_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log rows cannot be modified';
END
$$;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
