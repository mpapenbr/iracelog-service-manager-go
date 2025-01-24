BEGIN;
alter table event add column ir_sub_session_id integer not null default 0;
COMMIT;
