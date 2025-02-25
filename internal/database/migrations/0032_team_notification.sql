-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION api_notify () RETURNS trigger AS $$
BEGIN
  -- We accept a number of keys as arguments, and will read the values using NEW if it is set, or OLD if it is not.
  -- We will then send a notification to api_notifiy with a JSON object containing the keys and values, as well as
  -- the table name and operation.
  DECLARE
    values text[];
    i integer := 0;
    key text;
  BEGIN
    IF TG_NARGS > 0 AND TG_OP IN ('CREATE', 'UPDATE', 'DELETE') THEN
      FOREACH key IN ARRAY TG_ARGV LOOP
        IF TG_OP != 'DELETE' THEN
          values := array_append(values, row_to_json(NEW)->>key);
        ELSE
          values := array_append(values, row_to_json(OLD)->>key);
        END IF;
        i := i + 1;
      END LOOP;
    END IF;

    -- Construct the JSON object and send the notification. The JSON object will be of the form:
    -- {
    --   "table": "table_name",
    --   "op": "operation",
    --   "data": {
    --     "key1": "value1",
    --     "key2": "value2",
    --     ...
    --   }
    -- }
    PERFORM pg_notify('api_notify', jsonb_build_object('table', TG_TABLE_NAME, 'op', TG_OP, 'data', jsonb_object(TG_ARGV, values))::text);
    RETURN NULL;
  END;
RETURN NULL;
END;
$$ LANGUAGE plpgsql
;

-- +goose StatementEnd
CREATE
OR REPLACE TRIGGER teams_notify
AFTER INSERT
OR
UPDATE
OR DELETE ON teams FOR EACH ROW
EXECUTE PROCEDURE api_notify ("slug", "purpose")
;
