-- Tables of the identity-organization context (ADR-053): the aggregate in the
-- plural, tenant_id leading the key, a typed column only for what a query
-- filters by and the rest of the state in snapshot jsonb.
CREATE TABLE IF NOT EXISTS organizations (
  tenant_id       text   NOT NULL,
  organization_id text   NOT NULL,
  version         bigint NOT NULL DEFAULT 0,
  -- Typed because FindOrganizationByTaxId filters by the hash; the
  -- jurisdiction is typed because the constraint holds it.
  tax_id_hash     text   NOT NULL,
  jurisdiction    text   NOT NULL,
  snapshot        jsonb  NOT NULL,
  CONSTRAINT organizations_pkey PRIMARY KEY (tenant_id, organization_id),
  -- The authority over uniqueness per tenant (spec note ††): the Save turns
  -- its violation into the declared rejection, so a race ends here for the loser.
  CONSTRAINT organizations_tenant_id_tax_id_hash_jurisdiction_key UNIQUE (tenant_id, tax_id_hash, jurisdiction)
);

-- The cross-tenant probes of the kernel Table and Relation (IDN-12) look the
-- value up without the tenant, which an index led by tenant_id cannot serve.
CREATE INDEX IF NOT EXISTS organizations_organization_id_idx ON organizations (organization_id);
CREATE INDEX IF NOT EXISTS organizations_tax_id_hash_idx ON organizations (tax_id_hash);

CREATE TABLE IF NOT EXISTS establishments (
  tenant_id        text   NOT NULL,
  establishment_id text   NOT NULL,
  version          bigint NOT NULL DEFAULT 0,
  -- Typed because FindEstablishmentsByOrganization filters by the organization
  -- and the constraint holds the hash.
  organization_id  text   NOT NULL,
  tax_id_hash      text   NOT NULL,
  snapshot         jsonb  NOT NULL,
  CONSTRAINT establishments_pkey PRIMARY KEY (tenant_id, establishment_id),
  -- The authority over uniqueness per tenant (spec note ††), as for organizations.
  CONSTRAINT establishments_tenant_id_organization_id_tax_id_hash_key UNIQUE (tenant_id, organization_id, tax_id_hash)
);

CREATE INDEX IF NOT EXISTS establishments_tenant_id_tax_id_hash_idx ON establishments (tenant_id, tax_id_hash);
CREATE INDEX IF NOT EXISTS establishments_establishment_id_idx ON establishments (establishment_id);
CREATE INDEX IF NOT EXISTS establishments_organization_id_idx ON establishments (organization_id);
CREATE INDEX IF NOT EXISTS establishments_tax_id_hash_idx ON establishments (tax_id_hash);

-- The document itself is not stored in this version (spec note ‡‡): only its
-- hash, here, and its mask and key version, in the snapshot.
CREATE TABLE IF NOT EXISTS persons (
  tenant_id     text   NOT NULL,
  person_id     text   NOT NULL,
  version       bigint NOT NULL DEFAULT 0,
  document_hash text   NOT NULL,
  snapshot      jsonb  NOT NULL,
  CONSTRAINT persons_pkey PRIMARY KEY (tenant_id, person_id),
  CONSTRAINT persons_tenant_id_document_hash_key UNIQUE (tenant_id, document_hash)
);

CREATE INDEX IF NOT EXISTS persons_person_id_idx ON persons (person_id);
CREATE INDEX IF NOT EXISTS persons_document_hash_idx ON persons (document_hash);

CREATE TABLE IF NOT EXISTS representations (
  tenant_id         text   NOT NULL,
  representation_id text   NOT NULL,
  version           bigint NOT NULL DEFAULT 0,
  organization_id   text   NOT NULL,
  -- Typed with the organization as the spec declares the table; no query of
  -- this version filters by the person yet, so it has no index.
  person_id         text   NOT NULL,
  snapshot          jsonb  NOT NULL,
  CONSTRAINT representations_pkey PRIMARY KEY (tenant_id, representation_id)
);

CREATE INDEX IF NOT EXISTS representations_tenant_id_organization_id_idx ON representations (tenant_id, organization_id);
CREATE INDEX IF NOT EXISTS representations_representation_id_idx ON representations (representation_id);
CREATE INDEX IF NOT EXISTS representations_organization_id_idx ON representations (organization_id);

-- The subject is the natural identity (ADR-016) and the key of the kernel
-- Table, so the same subject holds one account per tenant.
CREATE TABLE IF NOT EXISTS user_accounts (
  tenant_id       text   NOT NULL,
  user_account_id text   NOT NULL,
  version         bigint NOT NULL DEFAULT 0,
  snapshot        jsonb  NOT NULL,
  CONSTRAINT user_accounts_pkey PRIMARY KEY (tenant_id, user_account_id)
);

CREATE INDEX IF NOT EXISTS user_accounts_user_account_id_idx ON user_accounts (user_account_id);

-- The invitee email is PII, kept in the snapshot without an extra envelope in
-- this version (spec note ‡‡).
CREATE TABLE IF NOT EXISTS memberships (
  tenant_id       text   NOT NULL,
  membership_id   text   NOT NULL,
  version         bigint NOT NULL DEFAULT 0,
  -- Typed because FindMembershipsByOrganization and FindMembershipsByUser
  -- filter by them; the user stays null until the grant.
  organization_id text   NOT NULL,
  user_id         text,
  snapshot        jsonb  NOT NULL,
  CONSTRAINT memberships_pkey PRIMARY KEY (tenant_id, membership_id)
);

CREATE INDEX IF NOT EXISTS memberships_tenant_id_organization_id_idx ON memberships (tenant_id, organization_id);
CREATE INDEX IF NOT EXISTS memberships_tenant_id_user_id_idx ON memberships (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS memberships_membership_id_idx ON memberships (membership_id);
CREATE INDEX IF NOT EXISTS memberships_organization_id_idx ON memberships (organization_id);
CREATE INDEX IF NOT EXISTS memberships_user_id_idx ON memberships (user_id);

-- Only the id of the evidence of the destination lives here: the bank data is
-- Funding's (docs/design/10 §4) and never reaches this context.
CREATE TABLE IF NOT EXISTS bank_account_changes (
  tenant_id              text   NOT NULL,
  bank_account_change_id text   NOT NULL,
  version                bigint NOT NULL DEFAULT 0,
  -- Typed because FindBankAccountChangesByOrganization filters by it.
  organization_id        text   NOT NULL,
  snapshot               jsonb  NOT NULL,
  CONSTRAINT bank_account_changes_pkey PRIMARY KEY (tenant_id, bank_account_change_id)
);

CREATE INDEX IF NOT EXISTS bank_account_changes_tenant_id_organization_id_idx ON bank_account_changes (tenant_id, organization_id);
CREATE INDEX IF NOT EXISTS bank_account_changes_bank_account_change_id_idx ON bank_account_changes (bank_account_change_id);
CREATE INDEX IF NOT EXISTS bank_account_changes_organization_id_idx ON bank_account_changes (organization_id);
