package domain

import "slices"

type PartnerStatus string

const (
	PartnerProspect     PartnerStatus = "prospect"
	PartnerContracting  PartnerStatus = "contracting"
	PartnerIntegrating  PartnerStatus = "integrating"
	PartnerHomologating PartnerStatus = "homologating"
	PartnerActive       PartnerStatus = "active"
	PartnerSuspended    PartnerStatus = "suspended"
	PartnerOffboarding  PartnerStatus = "offboarding"
)

var stageTransitions = map[PartnerStatus][]PartnerStatus{
	PartnerProspect:     {PartnerContracting},
	PartnerContracting:  {PartnerIntegrating},
	PartnerIntegrating:  {PartnerHomologating},
	PartnerHomologating: {PartnerActive},
	PartnerActive:       {PartnerSuspended, PartnerOffboarding},
	PartnerSuspended:    {PartnerActive, PartnerOffboarding},
}

func (s PartnerStatus) leadsTo(target PartnerStatus) bool {
	return slices.Contains(stageTransitions[s], target)
}

func (s PartnerStatus) demandsReason() bool {
	return s == PartnerSuspended || s == PartnerOffboarding
}
