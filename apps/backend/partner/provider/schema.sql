-- Tables of the partner-management context (ADR-053): the aggregate in the
-- plural, tenant_id leading the key, a typed column only for what a query
-- filters by and the rest of the state in snapshot jsonb.
--
-- WHY: a versioned series holds one first version. A UNIQUE constraint takes
-- no WHERE, so series_origin carries the series key on the first version alone
-- and NULL, which never collides, on every later one (ADR-066).
CREATE TABLE IF NOT EXISTS partners (
  tenant_id       text   NOT NULL,
  partner_id      text   NOT NULL,
  version         bigint NOT NULL DEFAULT 0,
  organization_id text   NOT NULL,
  role            text   NOT NULL,
  status          text   NOT NULL,
  snapshot        jsonb  NOT NULL,
  CONSTRAINT partners_pkey PRIMARY KEY (tenant_id, partner_id),
  CONSTRAINT partners_tenant_id_organization_id_role_key UNIQUE (tenant_id, organization_id, role)
);

CREATE INDEX IF NOT EXISTS partners_partner_id_idx ON partners (partner_id);
CREATE INDEX IF NOT EXISTS partners_organization_id_idx ON partners (organization_id);
CREATE INDEX IF NOT EXISTS partners_role_idx ON partners (role);

CREATE TABLE IF NOT EXISTS partner_agreements (
  tenant_id            text   NOT NULL,
  partner_agreement_id text   NOT NULL,
  version              bigint NOT NULL DEFAULT 0,
  partner_id           text   NOT NULL,
  status               text   NOT NULL,
  supersedes_id        text,
  snapshot             jsonb  NOT NULL,
  series_origin        text   GENERATED ALWAYS AS (CASE WHEN supersedes_id IS NULL THEN partner_id END) STORED,
  CONSTRAINT partner_agreements_pkey PRIMARY KEY (tenant_id, partner_agreement_id),
  CONSTRAINT partner_agreements_tenant_id_supersedes_id_key UNIQUE (tenant_id, supersedes_id),
  CONSTRAINT partner_agreements_tenant_id_series_origin_key UNIQUE (tenant_id, series_origin)
);

CREATE INDEX IF NOT EXISTS partner_agreements_partner_agreement_id_idx ON partner_agreements (partner_agreement_id);
CREATE INDEX IF NOT EXISTS partner_agreements_partner_id_idx ON partner_agreements (partner_id);

CREATE TABLE IF NOT EXISTS integration_profiles (
  tenant_id              text   NOT NULL,
  integration_profile_id text   NOT NULL,
  version                bigint NOT NULL DEFAULT 0,
  partner_id             text   NOT NULL,
  environment            text   NOT NULL,
  certificate_expires_at bigint NOT NULL,
  supersedes_id          text,
  snapshot               jsonb  NOT NULL,
  series_origin          text   GENERATED ALWAYS AS (CASE WHEN supersedes_id IS NULL THEN partner_id || chr(31) || environment END) STORED,
  CONSTRAINT integration_profiles_pkey PRIMARY KEY (tenant_id, integration_profile_id),
  CONSTRAINT integration_profiles_tenant_id_supersedes_id_key UNIQUE (tenant_id, supersedes_id),
  CONSTRAINT integration_profiles_tenant_id_series_origin_key UNIQUE (tenant_id, series_origin)
);

CREATE INDEX IF NOT EXISTS integration_profiles_integration_profile_id_idx ON integration_profiles (integration_profile_id);
CREATE INDEX IF NOT EXISTS integration_profiles_partner_id_idx ON integration_profiles (partner_id);
CREATE INDEX IF NOT EXISTS integration_profiles_environment_idx ON integration_profiles (environment);

CREATE TABLE IF NOT EXISTS operational_policies (
  tenant_id             text   NOT NULL,
  operational_policy_id text   NOT NULL,
  version               bigint NOT NULL DEFAULT 0,
  partner_id            text   NOT NULL,
  supersedes_id         text,
  snapshot              jsonb  NOT NULL,
  series_origin         text   GENERATED ALWAYS AS (CASE WHEN supersedes_id IS NULL THEN partner_id END) STORED,
  CONSTRAINT operational_policies_pkey PRIMARY KEY (tenant_id, operational_policy_id),
  CONSTRAINT operational_policies_tenant_id_supersedes_id_key UNIQUE (tenant_id, supersedes_id),
  CONSTRAINT operational_policies_tenant_id_series_origin_key UNIQUE (tenant_id, series_origin)
);

CREATE INDEX IF NOT EXISTS operational_policies_operational_policy_id_idx ON operational_policies (operational_policy_id);
CREATE INDEX IF NOT EXISTS operational_policies_partner_id_idx ON operational_policies (partner_id);

CREATE TABLE IF NOT EXISTS capabilities (
  tenant_id     text   NOT NULL,
  capability_id text   NOT NULL,
  version       bigint NOT NULL DEFAULT 0,
  partner_id    text   NOT NULL,
  operation     text   NOT NULL,
  asset_type    text   NOT NULL,
  environment   text   NOT NULL,
  status        text   NOT NULL,
  supersedes_id text,
  snapshot      jsonb  NOT NULL,
  series_origin text   GENERATED ALWAYS AS (CASE WHEN supersedes_id IS NULL THEN partner_id || chr(31) || operation || chr(31) || asset_type || chr(31) || environment END) STORED,
  CONSTRAINT capabilities_pkey PRIMARY KEY (tenant_id, capability_id),
  CONSTRAINT capabilities_tenant_id_supersedes_id_key UNIQUE (tenant_id, supersedes_id),
  CONSTRAINT capabilities_tenant_id_series_origin_key UNIQUE (tenant_id, series_origin)
);

CREATE INDEX IF NOT EXISTS capabilities_capability_id_idx ON capabilities (capability_id);
CREATE INDEX IF NOT EXISTS capabilities_partner_id_idx ON capabilities (partner_id);

CREATE TABLE IF NOT EXISTS financial_products (
  tenant_id            text   NOT NULL,
  financial_product_id text   NOT NULL,
  version              bigint NOT NULL DEFAULT 0,
  partner_id           text   NOT NULL,
  code                 text   NOT NULL,
  status               text   NOT NULL,
  valid_until          bigint NOT NULL,
  supersedes_id        text,
  snapshot             jsonb  NOT NULL,
  series_origin        text   GENERATED ALWAYS AS (CASE WHEN supersedes_id IS NULL THEN partner_id || chr(31) || code END) STORED,
  CONSTRAINT financial_products_pkey PRIMARY KEY (tenant_id, financial_product_id),
  CONSTRAINT financial_products_tenant_id_supersedes_id_key UNIQUE (tenant_id, supersedes_id),
  CONSTRAINT financial_products_tenant_id_series_origin_key UNIQUE (tenant_id, series_origin)
);

CREATE INDEX IF NOT EXISTS financial_products_financial_product_id_idx ON financial_products (financial_product_id);
CREATE INDEX IF NOT EXISTS financial_products_partner_id_idx ON financial_products (partner_id);
