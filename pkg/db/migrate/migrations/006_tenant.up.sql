BEGIN;
CREATE TABLE IF NOT EXISTS tenant (
	id serial,
	external_id uuid not null,
	name varchar not null,
	api_key varchar not null,
	active boolean not null default false
);
COMMENT ON TABLE tenant IS 'Information about a tenant';
COMMENT ON COLUMN tenant.external_id IS 'for external use';



-- Column id is associated with sequence public.tenant_id_seq
ALTER TABLE
	tenant
ADD
	CONSTRAINT tenant_pkey PRIMARY KEY (id),
ADD
	CONSTRAINT tenant_external_id_unique UNIQUE (external_id),
ADD
	CONSTRAINT tenant_name_unique UNIQUE (name),
ADD
	CONSTRAINT tenant_api_key_unique UNIQUE (api_key)
;

ALTER TABLE event ADD column tenant_id integer;
WITH new_tenant AS (
	insert into tenant (name,external_id,api_key)
	 values ('default',uuid_generate_v4(),'') returning id
) UPDATE event SET tenant_id=(SELECT id FROM new_tenant);
ALTER TABLE event ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE event ADD CONSTRAINT event_tenant_id_fk FOREIGN KEY (tenant_id) REFERENCES tenant(id);
COMMIT;
