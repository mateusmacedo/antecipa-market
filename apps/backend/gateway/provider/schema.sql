-- Tables of the receivables-gateway context (ADR-053): the aggregate in the
-- plural, tenant_id leading the key, a typed column only for what a query
-- filters by and the rest of the state in snapshot jsonb. The attempt ledger
-- external_attempt is the kernel's; the raw payloads never live here.
CREATE TABLE IF NOT EXISTS connectors (
  tenant_id    text   NOT NULL,
  connector_id text   NOT NULL,
  version      bigint NOT NULL DEFAULT 0,
  snapshot     jsonb  NOT NULL,
  CONSTRAINT connectors_pkey PRIMARY KEY (tenant_id, connector_id)
);

CREATE INDEX IF NOT EXISTS connectors_connector_id_idx ON connectors (connector_id);

-- connector_id is the guard of CreateExternalCommand's read of the profile in
-- force (ADR-067); version is typed for FindEffectiveCapabilityProfile.
CREATE TABLE IF NOT EXISTS capability_profiles (
  tenant_id             text    NOT NULL,
  capability_profile_id text    NOT NULL,
  version               bigint  NOT NULL DEFAULT 0,
  connector_id          text    NOT NULL,
  profile_version       integer NOT NULL,
  snapshot              jsonb   NOT NULL,
  CONSTRAINT capability_profiles_pkey PRIMARY KEY (tenant_id, capability_profile_id)
);

CREATE INDEX IF NOT EXISTS capability_profiles_tenant_id_connector_id_idx ON capability_profiles (tenant_id, connector_id);
CREATE INDEX IF NOT EXISTS capability_profiles_capability_profile_id_idx ON capability_profiles (capability_profile_id);
CREATE INDEX IF NOT EXISTS capability_profiles_connector_id_idx ON capability_profiles (connector_id);

CREATE TABLE IF NOT EXISTS query_authorizations (
  tenant_id              text  NOT NULL,
  query_authorization_id text  NOT NULL,
  version                bigint NOT NULL DEFAULT 0,
  organization_id        text  NOT NULL,
  snapshot               jsonb NOT NULL,
  CONSTRAINT query_authorizations_pkey PRIMARY KEY (tenant_id, query_authorization_id)
);

CREATE INDEX IF NOT EXISTS query_authorizations_tenant_id_organization_id_idx ON query_authorizations (tenant_id, organization_id);
CREATE INDEX IF NOT EXISTS query_authorizations_query_authorization_id_idx ON query_authorizations (query_authorization_id);
CREATE INDEX IF NOT EXISTS query_authorizations_organization_id_idx ON query_authorizations (organization_id);

-- series_key is the guard of the series check (ADR-067); outbound_key finds
-- the command of an attempt the kernel's sweep abandons.
CREATE TABLE IF NOT EXISTS external_commands (
  tenant_id           text   NOT NULL,
  external_command_id text   NOT NULL,
  version             bigint NOT NULL DEFAULT 0,
  connector_id        text   NOT NULL,
  business_key        text   NOT NULL,
  series_key          text   NOT NULL,
  status              text   NOT NULL,
  outbound_key        text   NOT NULL,
  snapshot            jsonb  NOT NULL,
  CONSTRAINT external_commands_pkey PRIMARY KEY (tenant_id, external_command_id)
);

CREATE INDEX IF NOT EXISTS external_commands_tenant_id_business_key_idx ON external_commands (tenant_id, business_key);
CREATE INDEX IF NOT EXISTS external_commands_tenant_id_series_key_idx ON external_commands (tenant_id, series_key);
CREATE INDEX IF NOT EXISTS external_commands_tenant_id_status_idx ON external_commands (tenant_id, status);
CREATE INDEX IF NOT EXISTS external_commands_tenant_id_outbound_key_idx ON external_commands (tenant_id, outbound_key);
CREATE INDEX IF NOT EXISTS external_commands_external_command_id_idx ON external_commands (external_command_id);
CREATE INDEX IF NOT EXISTS external_commands_business_key_idx ON external_commands (business_key);
CREATE INDEX IF NOT EXISTS external_commands_series_key_idx ON external_commands (series_key);
CREATE INDEX IF NOT EXISTS external_commands_status_idx ON external_commands (status);
CREATE INDEX IF NOT EXISTS external_commands_outbound_key_idx ON external_commands (outbound_key);
