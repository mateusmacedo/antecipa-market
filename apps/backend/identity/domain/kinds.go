package domain

import "slices"

type OrganizationKind string

const (
	KindClient   OrganizationKind = "client"
	KindFunder   OrganizationKind = "funder"
	KindPartner  OrganizationKind = "partner"
	KindPlatform OrganizationKind = "platform"
)

var organizationKinds = [...]OrganizationKind{KindClient, KindFunder, KindPartner, KindPlatform}

// OrganizationKinds is a set rather than a slice so the aggregate state and the
// event stay comparable values: the kernel testkit compares events with ==,
// and a slice field would panic there.
type OrganizationKinds struct{ bits uint8 }

// KindsOf drops values outside the closed catalogue, so an unknown kind can
// only ever make the set smaller.
func KindsOf(kinds ...OrganizationKind) OrganizationKinds {
	var set OrganizationKinds
	for _, kind := range kinds {
		if i := slices.Index(organizationKinds[:], kind); i >= 0 {
			set.bits |= 1 << i
		}
	}
	return set
}

func (s OrganizationKinds) Has(kind OrganizationKind) bool {
	other := KindsOf(kind)
	return other.bits != 0 && s.bits&other.bits != 0
}

func (s OrganizationKinds) Empty() bool { return s.bits == 0 }

func (s OrganizationKinds) List() []OrganizationKind {
	out := make([]OrganizationKind, 0, len(organizationKinds))
	for i, kind := range organizationKinds {
		if s.bits&(1<<i) != 0 {
			out = append(out, kind)
		}
	}
	return out
}
