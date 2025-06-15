BEGIN;
alter table event add column tire_infos jsonb ;
COMMIT;
