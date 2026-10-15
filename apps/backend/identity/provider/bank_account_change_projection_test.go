package provider_test

import (
	"strconv"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var bankAccountChangeStatusNames = map[domain.BankAccountChangeStatus]string{
	domain.BankAccountChangeStatusNew:       "new",
	domain.BankAccountChangeStatusRequested: "requested",
	domain.BankAccountChangeStatusApproved:  "approved",
}

func TestBankAccountChangesMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "bank-account-change.golden")
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "bank-account-change" || len(f.Cases) != 7 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runBankAccountChange)
}

func runBankAccountChange(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	at := kernel.Instant(integer(t, c.Command["at"]))
	response := func(id domain.BankAccountChangeID) domainkit.Fields {
		return domainkit.Fields{"bank_account_change": string(id)}
	}
	switch c.Command["upr"] {
	case "request":
		s := bankAccountChangeSubject(func(b *domain.BankAccountChange) (kernel.Accepted[domain.RequestBankAccountChangeResponse], *kernel.Rejection) {
			return b.Request(domain.RequestBankAccountChange{
				OrganizationID:        domain.OrganizationID(c.Command["organization"]),
				OrganizationStatus:    statusFrom(c.Command["organization_status"]),
				DestinationEvidenceID: c.Command["destination_evidence"],
				RequestedByUserID:     c.Command["requested_by"],
				At:                    at,
			})
		}, func(r domain.RequestBankAccountChangeResponse) domainkit.Fields {
			return response(r.BankAccountChangeID)
		})
		return domainkit.Run(bankAccountChangeFromState(t, c.StateBefore), s), domainkit.ReadTwice(bankAccountChangeFromState(t, c.StateBefore), s)
	case "approve":
		s := bankAccountChangeSubject(func(b *domain.BankAccountChange) (kernel.Accepted[domain.ApproveBankAccountChangeResponse], *kernel.Rejection) {
			return b.Approve(domain.ApproveBankAccountChange{ApprovedByUserID: c.Command["approved_by"], At: at})
		}, func(r domain.ApproveBankAccountChangeResponse) domainkit.Fields {
			return response(r.BankAccountChangeID)
		})
		return domainkit.Run(bankAccountChangeFromState(t, c.StateBefore), s), domainkit.ReadTwice(bankAccountChangeFromState(t, c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func bankAccountChangeState(b *domain.BankAccountChange) domainkit.Fields {
	s := b.Snapshot()
	return domainkit.Fields{
		"bank_account_change":  string(s.ID),
		"organization":         string(s.OrganizationID),
		"destination_evidence": s.DestinationEvidenceID,
		"requested_by":         s.RequestedByUserID,
		"approved_by":          s.ApprovedByUserID,
		"status":               bankAccountChangeStatusNames[s.Status],
		"requested_at":         strconv.FormatInt(int64(s.RequestedAt), 10),
		"approved_at":          strconv.FormatInt(int64(s.ApprovedAt), 10),
	}
}

func bankAccountChangeFromState(t *testing.T, f domainkit.Fields) *domain.BankAccountChange {
	t.Helper()
	s := domain.BankAccountChangeSnapshot{
		ID:                    domain.BankAccountChangeID(f["bank_account_change"]),
		OrganizationID:        domain.OrganizationID(f["organization"]),
		DestinationEvidenceID: f["destination_evidence"],
		RequestedByUserID:     f["requested_by"],
		ApprovedByUserID:      f["approved_by"],
		RequestedAt:           kernel.Instant(integer(t, f["requested_at"])),
		ApprovedAt:            kernel.Instant(integer(t, f["approved_at"])),
	}
	for status, name := range bankAccountChangeStatusNames {
		if name == f["status"] {
			s.Status = status
		}
	}
	return domain.FromBankAccountChangeSnapshot(s)
}

func bankAccountChangeEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch event := e.(type) {
	case domain.BankAccountChangeRequested:
		return event.EventName(), domainkit.Fields{
			"bank_account_change":  string(event.BankAccountChangeID),
			"organization":         string(event.OrganizationID),
			"destination_evidence": event.DestinationEvidenceID,
			"requested_by":         event.RequestedByUserID,
			"at":                   strconv.FormatInt(int64(event.At), 10),
		}
	case domain.BankAccountChangeApproved:
		return event.EventName(), domainkit.Fields{
			"bank_account_change": string(event.BankAccountChangeID),
			"organization":        string(event.OrganizationID),
			"approved_by":         event.ApprovedByUserID,
			"at":                  strconv.FormatInt(int64(event.At), 10),
		}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func bankAccountChangeSubject[R any](decide func(*domain.BankAccountChange) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.BankAccountChange, R] {
	return domainkit.Subject[*domain.BankAccountChange, R]{
		Decide:   decide,
		Response: response,
		Event:    bankAccountChangeEvent,
		Snapshot: bankAccountChangeState,
		Clone: func(b *domain.BankAccountChange) *domain.BankAccountChange {
			return domain.FromBankAccountChangeSnapshot(b.Snapshot())
		},
	}
}
