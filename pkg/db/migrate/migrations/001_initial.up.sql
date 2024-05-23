-- This entry describes the database tables as of 2023-02-19
-- In production these tables exist in the way they are defined here
-- We need this migration step only for setting up fresh databases
BEGIN;

-- ### required extension for generating uuid ###
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ### track ###
CREATE TABLE IF NOT EXISTS track (
	id serial,
	name varchar not null,
	short_name varchar not null,
	config varchar not null,
	track_length numeric not null,
	sectors jsonb not null,
	pit_speed numeric not null default 0,
	pit_entry numeric not null default 0,
	pit_exit numeric not null default 0,
	pit_lane_length numeric not null default 0
);

-- Column id is associated with sequence public.track_id_seq
ALTER TABLE
	track
ADD
	CONSTRAINT track_pkey PRIMARY KEY (id);

-- ### event ###
CREATE TABLE IF NOT EXISTS event (
	id serial,
	event_key varchar not null,
	name varchar not null,
	description varchar,
	event_time timestamp with time zone DEFAULT now () NOT NULL,
	racelogger_version varchar not null,
	team_racing boolean not null default false,
	multi_class boolean not null default false,
	num_car_types integer not null default 0,
	num_car_classes integer not null default 0,
	ir_session_id integer,
	track_id integer not null,
	pit_speed numeric not null default 0,
	replay_min_timestamp timestamp with time zone DEFAULT now () NOT NULL,
	replay_min_session_time numeric not null default 0,
	replay_max_session_time numeric not null default 0,
	sessions jsonb not null

);
comment on table event is 'Information about a recorded event';
comment on column event.replay_min_timestamp is 'timestamp of the race start';
comment on column event.replay_min_session_time is 'session time of the race start';
comment on column event.replay_max_session_time is 'session time of the race end';


-- Column id is associated with sequence public.event_id_seq
ALTER TABLE
	event
ADD
	CONSTRAINT event_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT event_event_key_key UNIQUE (event_key),
ADD
	CONSTRAINT event_track_id_fkey FOREIGN KEY (track_id) REFERENCES track (id);


-- ### event_ext ###
CREATE TABLE IF NOT EXISTS event_ext (
	event_id integer not null,
	extra_info jsonb not null default '{}'
);
ALTER TABLE
	event_ext
ADD
	CONSTRAINT event_ext_pkey PRIMARY KEY (event_id),
ADD
	CONSTRAINT event_ext_event_id_fkey FOREIGN KEY (event_id) REFERENCES event (id);

comment on table event_ext is 'additional information about a recorded event';


-- ### rs_info (record stamp info) ###
CREATE TABLE IF NOT EXISTS rs_info (
	id serial,
	event_id integer not null,
	record_stamp timestamp with time zone DEFAULT now () NOT NULL,
	session_time numeric not null,
	time_of_day integer not null,
	air_temp numeric not null,
	track_temp numeric not null,
	track_wetness integer not null,
	precipitation numeric not null
);
comment on table rs_info is 'Shared information for a recorded race state';

-- Column id is associated with sequence public.rs_info_id_seq
ALTER TABLE
	rs_info
ADD
	CONSTRAINT rs_info_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT rs_info_event_id_fkey FOREIGN KEY (event_id) REFERENCES event(id);

CREATE INDEX IF NOT EXISTS rs_info_event_id_idx ON rs_info (event_id);


-- ### race state proto ###
CREATE TABLE IF NOT EXISTS race_state_proto (
	id serial,
	rs_info_id integer not null,
	protodata bytea not null
);

-- Column id is associated with sequence public.race_state_proto_id_seq
ALTER TABLE
	race_state_proto
ADD
	CONSTRAINT race_state_proto_pkey PRIMARY KEY (id);

ALTER TABLE
	race_state_proto
ADD
	CONSTRAINT race_state_proto_rs_info_id_fkey FOREIGN KEY (rs_info_id) REFERENCES rs_info (id);

-- ### car state proto ###
CREATE TABLE IF NOT EXISTS msg_state_proto (
	id serial,
	rs_info_id integer not null,
	protodata bytea not null
);
comment on table msg_state_proto is 'Messages extracted from race_state_proto';
-- Column id is associated with sequence public.msg_state_proto_id_seq
ALTER TABLE
	msg_state_proto
ADD
	CONSTRAINT msg_state_proto_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT msg_state_proto_rs_info_id_fkey FOREIGN KEY (rs_info_id) REFERENCES rs_info (id);


-- ### car state proto ###
CREATE TABLE IF NOT EXISTS car_state_proto (
	id serial,
	rs_info_id integer not null,
	protodata bytea not null
);

-- Column id is associated with sequence public.car_state_proto_id_seq
ALTER TABLE
	car_state_proto
ADD
	CONSTRAINT car_state_proto_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT car_state_proto_rs_info_id_fkey FOREIGN KEY (rs_info_id) REFERENCES rs_info (id);

-- ### speemdap proto ###
CREATE TABLE IF NOT EXISTS speedmap_proto (
	id serial,
	rs_info_id integer not null,
	protodata bytea not null
);

-- Column id is associated with sequence public.speedmap_proto_id_seq
ALTER TABLE
	speedmap_proto
ADD
	CONSTRAINT speedmap_proto_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT speedmap_proto_rs_info_id_fkey FOREIGN KEY (rs_info_id) REFERENCES rs_info (id);

-- ### analysis_proto (binary protobuf data containing analysis data) ###
CREATE TABLE IF NOT EXISTS analysis_proto (
	event_id integer not null,
	record_stamp timestamp with time zone DEFAULT now () NOT NULL,
	protodata bytea not null

);
comment on table analysis_proto is 'Analysis data in binary protobuf format';
comment on column analysis_proto.record_stamp is 'Timestamp when data was persisted';

ALTER TABLE
	analysis_proto
ADD
	CONSTRAINT analysis_proto_pkey PRIMARY KEY (event_id),
ADD
	CONSTRAINT analysis_proto_event_id_fkey FOREIGN KEY (event_id) REFERENCES event (id);

-- ### c_car_class (car class data) ###
CREATE TABLE IF NOT EXISTS c_car_class (
	id serial,
	event_id integer not null,
	name varchar not null,
	car_class_id integer not null
);

-- Column id is associated with sequence public.c_car_class_id_seq
ALTER TABLE
	c_car_class
ADD
	CONSTRAINT c_car_class_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT c_car_class_event_id_fkey FOREIGN KEY (event_id) REFERENCES event (id);

-- ### c_car (car data) ###
CREATE TABLE IF NOT EXISTS c_car (
	id serial,
	event_id integer not null,
	name varchar not null,
	name_short varchar not null,
	car_id integer not null,
	c_car_class_id integer not null,
	fuel_pct numeric not null default 1,
	power_adjust numeric not null default 0,
	weight_penalty numeric not null default 0,
	dry_tire_sets integer not null default 0
);

-- Column id is associated with sequence public.c_car_id_seq
ALTER TABLE
	c_car
ADD
	CONSTRAINT c_car_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT c_car_event_id_fkey FOREIGN KEY (event_id) REFERENCES event (id),
ADD
	CONSTRAINT c_car_class_id_fkey FOREIGN KEY (c_car_class_id) REFERENCES c_car_class (id);

-- ### c_car_entry (car entry data) ###
CREATE TABLE IF NOT EXISTS c_car_entry (
	id serial,
	event_id integer not null,
	c_car_id integer not null,
	car_idx integer not null,
	car_number varchar not null,
	car_number_raw integer not null
);

-- Column id is associated with sequence public.c_car_id_seq
ALTER TABLE
	c_car_entry
ADD
	CONSTRAINT c_car_entry_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT c_car_entry_event_id_fkey FOREIGN KEY (event_id) REFERENCES event (id),
ADD
	CONSTRAINT c_car_entry_car_id_fkey FOREIGN KEY (c_car_id) REFERENCES c_car (id);

-- ### c_car_team (car team data) ###
CREATE TABLE IF NOT EXISTS c_car_team (
	id serial,
	c_car_entry_id integer not null,
	team_id integer not null,
	name varchar not null
);

-- Column id is associated with sequence public.c_car_id_seq
ALTER TABLE
	c_car_team
ADD
	CONSTRAINT c_car_team_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT c_car_team_car_entry_id_fkey FOREIGN KEY (c_car_entry_id) REFERENCES c_car_entry (id);

-- ### c_car_driver (car driver data) ###
CREATE TABLE IF NOT EXISTS c_car_driver (
	id serial,
	c_car_entry_id integer not null,
	driver_id integer not null,
	name varchar not null,
	initials varchar not null,
	abbrev_name varchar not null,
	irating integer not null,
	lic_level integer not null,
	lic_sub_level integer not null,
	lic_string varchar not null
);

-- Column id is associated with sequence public.c_car_driver_id_seq
ALTER TABLE
	c_car_driver
ADD
	CONSTRAINT c_car_driver_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT c_car_driver_car_entry_id_fkey FOREIGN KEY (c_car_entry_id) REFERENCES c_car_entry (id);




COMMIT;
