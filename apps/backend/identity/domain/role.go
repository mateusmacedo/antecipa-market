package domain

type Role string

const (
	RoleOperator    Role = "operator"
	RoleAuditor     Role = "auditor"
	RolePlatformOps Role = "platform_ops"
)

type VerificationLevel string

const (
	VerificationChannelOwnership    VerificationLevel = "channel_ownership"
	VerificationPersonalDataChecked VerificationLevel = "personal_data_checked"
)

var verificationRanks = map[VerificationLevel]int{
	VerificationChannelOwnership:    1,
	VerificationPersonalDataChecked: 2,
}

var minimumVerification = map[Role]VerificationLevel{
	RoleOperator:    VerificationPersonalDataChecked,
	RoleAuditor:     VerificationChannelOwnership,
	RolePlatformOps: VerificationPersonalDataChecked,
}

var requiredKind = map[Role]OrganizationKind{
	RoleOperator:    KindClient,
	RoleAuditor:     KindClient,
	RolePlatformOps: KindPlatform,
}

// admits answers false for a role outside the catalogue, so an unknown role is
// refused as incompatible instead of entering with no kind to check.
func (r Role) admits(kinds OrganizationKinds) bool {
	kind, known := requiredKind[r]
	return known && kinds.Has(kind)
}

// verifiedBy answers false for a level or a role outside the catalogue: an
// unknown level ranks below every minimum and an unknown role has none to reach.
func (r Role) verifiedBy(level VerificationLevel) bool {
	minimum, known := minimumVerification[r]
	return known && verificationRanks[level] >= verificationRanks[minimum]
}
