BEGIN;
alter table event drop column tenant_id;
drop table if exists tenant;
COMMIT;
