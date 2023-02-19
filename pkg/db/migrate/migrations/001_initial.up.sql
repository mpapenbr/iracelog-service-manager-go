-- This entry describes the database tables as of 2023-02-19
-- In production these tables exist in the way they are defined here
-- We need this migration step only for setting up fresh databases

BEGIN;

-- ### required extension for generating uuid ###
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ### event ###

CREATE TABLE IF NOT EXISTS event
(
   id            serial,
   event_key     varchar,
   name          varchar,
   data          jsonb,
   description   varchar,
   record_stamp  timestamp   DEFAULT now() NOT NULL
);

-- Column id is associated with sequence public.event_id_seq

ALTER TABLE event
   ADD CONSTRAINT event_pkey
   PRIMARY KEY (id);

ALTER TABLE event
   ADD CONSTRAINT event_event_key_key UNIQUE (event_key);


-- ### analysis ###
CREATE TABLE IF NOT EXISTS analysis
(
   id        serial,
   event_id  integer   NOT NULL,
   data      jsonb
);

-- Column id is associated with sequence public.analysis_id_seq

ALTER TABLE analysis
   ADD CONSTRAINT analysis_pkey
   PRIMARY KEY (id);

ALTER TABLE analysis
  ADD CONSTRAINT analysis_event_id_fkey
  FOREIGN KEY (event_id) REFERENCES event(id);

-- ### car ###

CREATE TABLE IF NOT EXISTS car
(
   id        integer,
   event_id  integer   NOT NULL,
   data      jsonb
);

ALTER TABLE car
   ADD CONSTRAINT driver_pkey
   PRIMARY KEY (id);

ALTER TABLE car
  ADD CONSTRAINT driver_event_id_fkey
  FOREIGN KEY (event_id) REFERENCES event(id);

-- ### event_ext ###

CREATE TABLE IF NOT EXISTS event_ext
(
   id        serial,
   event_id  integer   NOT NULL,
   data      jsonb
);

-- Column id is associated with sequence public.event_ext_id_seq

ALTER TABLE event_ext
   ADD CONSTRAINT event_ext_pkey
   PRIMARY KEY (id);

ALTER TABLE event_ext
  ADD CONSTRAINT event_ext_event_id_fkey
  FOREIGN KEY (event_id) REFERENCES event(id);

-- ### speedmap ###

CREATE TABLE IF NOT EXISTS speedmap
(
   id        serial,
   event_id  integer   NOT NULL,
   data      jsonb
);

-- Column id is associated with sequence public.speedmap_id_seq

ALTER TABLE speedmap
   ADD CONSTRAINT speedmap_pkey
   PRIMARY KEY (id);

ALTER TABLE speedmap
  ADD CONSTRAINT speedmap_event_id_fkey
  FOREIGN KEY (event_id) REFERENCES event(id);

-- ### track ###

CREATE TABLE IF NOT EXISTS track
(
   id    serial,
   data  jsonb
);

-- Column id is associated with sequence public.track_id_seq

ALTER TABLE track
   ADD CONSTRAINT track_pkey
   PRIMARY KEY (id);

-- ### wampdata ###

CREATE TABLE IF NOT EXISTS wampdata
(
   id        serial,
   event_id  integer   NOT NULL,
   data      jsonb
);

-- Column id is associated with sequence public.wampdata_id_seq

ALTER TABLE wampdata
   ADD CONSTRAINT wampdata_pkey
   PRIMARY KEY (id);

ALTER TABLE wampdata
  ADD CONSTRAINT wampdata_event_id_fkey
  FOREIGN KEY (event_id) REFERENCES event(id);

CREATE INDEX IF NOT EXISTS ix_wampdata_timestamp ON public.wampdata USING btree ((((data -> 'timestamp'::text))::numeric));

CREATE INDEX IF NOT EXISTS ix_wampdata_event_id ON public.wampdata USING btree (event_id);

-- Stored procedures

-- jsonb_merge

CREATE OR REPLACE FUNCTION public.mgm_jsonb_merge(p_val_1 jsonb, p_val_2 jsonb)
  RETURNS jsonb
  LANGUAGE sql
AS
$body$
/**********************************************************************
 Deep merges two JSON values
 ===========================

 For scalar values, arrays or values of different types, the value from the second argument overwrites
 the value from the first argument (for the same key).

 For "JSON objects, the function recursively merges the values for each key.

 If the value for a key in the second argument is null, the key and it's value
 will be removed completely.

 Examples
 ========

 Simple key/value
 ----------------
 mgm_jsonb_merge('{"answer": 1}',
                 '{"answer": 42}')

 returns: {"answer": 42}

 Non-matching types
 ------------------

 mgm_jsonb_merge('{"answer": 42}',
                 '{"answer": {"value": 42}}')

 returns: {"answer": {"value": 42}}

 Nested types and arrays
 -----------------------

 mgm_jsonb_merge('{"answer": 1, "foo": {"one": "x", "two": "y"}, "names": ["one","two"]}',
                 '{"answer": 42, "foo": {"three": "z"}, "names": ["three"]}')

 returns: {"answer": 42, "foo": {"one": "x", "two": "y", "three": "z"}, "names": ["three"]}


 Removing keys
 -------------

 mgm_jsonb_merge('{"answer": 1, "foo": {"one": "x", "two": "y"}}',
                 '{"foo": {"two": null}}')

 returns: {"answer": 1, "foo": {"one": "x"}}
*********************************************************************/

  -- if either value is NULL, return the other
  -- don't do any recursive processing
  select coalesce(p_val_1, p_val_2)
  where p_val_1 is null or p_val_2 is null

  union all

  select
    -- strip_nulls makes sure we can delete keys from the JSON
    -- by setting them to null
    jsonb_strip_nulls(
      jsonb_object_agg(
        -- as this is a full outer join, one of the keys could be null
        coalesce(ka, kb),
          -- process the value for the current key
          -- if either value is null, just use the not null one
          case
            when va is null then vb
            when vb is null then va

            -- both values are objects, merge them recursively
            when jsonb_typeof(va) = 'object' and jsonb_typeof(vb) = 'object'
              then mgm_jsonb_merge(va, vb)

            -- the two values are not of the same type or are arrays, so return the second one
            -- this overwrites values from the first argument with values from the second
            else vb
          end
      ) -- end of jsonb_object_agg()
    )  -- end of jsonb_strip_nulls()
  from jsonb_each(p_val_1) e1(ka, va)
    full join jsonb_each(p_val_2) e2(kb, vb) on ka = kb
  where p_val_1 is not null and p_val_2 is not null;
$body$
  IMMUTABLE
  COST 100
  PARALLEL SAFE;




COMMIT;
