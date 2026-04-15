CREATE OR REPLACE FUNCTION prevent_audit_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit log is immutable (updates/deletes forbidden)';
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER audit_immutable_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

CREATE TRIGGER audit_immutable_delete
    BEFORE DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

ALTER TABLE audit_log ADD CONSTRAINT audit_log_event_id_unique UNIQUE (event_id);
