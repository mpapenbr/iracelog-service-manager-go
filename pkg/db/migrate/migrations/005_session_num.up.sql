BEGIN;
alter table rs_info add column session_num integer not null default 0;
COMMIT;
