package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

var referenceKey = domain.ReferenceKeyOf(partnerID, domain.SystemRegistry, domain.ReferenceAsset, " dup-0001/a ")

func link() domain.LinkExternalReference {
	return domain.LinkExternalReference{
		ProviderID: partnerID, System: domain.SystemRegistry, ReferenceType: domain.ReferenceAsset, Reference: " dup-0001/a ",
		SubjectType: "asset", SubjectID: "ast-1", AsOf: at, ObservedVersion: "v1",
	}
}

func linked(t *testing.T) *domain.ExternalReference {
	t.Helper()
	r := domain.NewExternalReference(referenceKey)
	if _, rej := r.Link(link()); rej != nil {
		t.Fatalf("setup: Link rejected: %v", rej)
	}
	return r
}

func TestTheReferenceKeyIsTheProviderTheSystemTheTypeAndTheReferenceAsReceived(t *testing.T) {
	if referenceKey != "prt-1:registry:asset: dup-0001/a " {
		t.Fatalf("ReferenceKeyOf() = %q", referenceKey)
	}
}

func TestLinkKeepsTheReferenceAsReceivedBesideTheDerivedForm(t *testing.T) {
	r := domain.NewExternalReference(referenceKey)

	acc, rej := r.Link(link())

	tb.RequireAccepted(t, rej)
	if acc.Response().ReferenceKey != referenceKey {
		t.Fatalf("Response() = %+v", acc.Response())
	}
	want := domain.ExternalReferenceSnapshot{
		Key: referenceKey, ProviderID: partnerID, System: domain.SystemRegistry, ReferenceType: domain.ReferenceAsset,
		Reference: " dup-0001/a ", NormalizedReference: "DUP-0001/A", SubjectType: "asset", SubjectID: "ast-1", AsOf: at, ObservedVersion: "v1",
	}
	if got := r.Snapshot(); !got.Equal(want) {
		t.Fatalf("snapshot = %+v, want %+v", got, want)
	}
	requireNoEvents(t, acc)
}

func TestLinkKeepsTheNormalizedFormTheAdapterDerived(t *testing.T) {
	cmd := link()
	cmd.NormalizedReference = "DUP0001A"
	r := domain.NewExternalReference(referenceKey)

	_, rej := r.Link(cmd)

	tb.RequireAccepted(t, rej)
	if s := r.Snapshot(); s.NormalizedReference != "DUP0001A" || s.Reference != " dup-0001/a " {
		t.Fatalf("snapshot = %+v, want the adapter's normalized form beside the original", s)
	}
}

func TestLinkToTheSameObjectRenewsOnlyAMoreRecentObservation(t *testing.T) {
	r := linked(t)
	newer := link()
	newer.AsOf, newer.ObservedVersion = at+second, "v2"

	_, rej := r.Link(newer)

	tb.RequireAccepted(t, rej)
	if s := r.Snapshot(); s.AsOf != at+second || s.ObservedVersion != "v2" {
		t.Fatalf("snapshot = %+v, want asOf and observedVersion renewed", s)
	}

	older := link()
	older.AsOf, older.ObservedVersion = at, "v0"
	before := r.Snapshot()
	_, rej = r.Link(older)

	tb.RequireAccepted(t, rej)
	if after := r.Snapshot(); !before.Equal(after) {
		t.Fatalf("an older observation changed the link: %+v", after)
	}
}

func TestLinkToAnotherObjectIsRefused(t *testing.T) {
	r := linked(t)
	before := r.Snapshot()
	other := link()
	other.SubjectID = "ast-2"

	acc, rej := r.Link(other)

	tb.RequireRejected(t, acc, rej, domain.CodeExternalReferenceAlreadyLinked)
	if after := r.Snapshot(); !before.Equal(after) {
		t.Fatalf("a refused link changed the reference: %+v", after)
	}
}

func TestLinkWithAnEmptyReferenceIsRefused(t *testing.T) {
	cmd := link()
	cmd.Reference = ""
	r := domain.NewExternalReference(domain.ReferenceKeyOf(partnerID, domain.SystemRegistry, domain.ReferenceAsset, ""))

	acc, rej := r.Link(cmd)

	tb.RequireRejected(t, acc, rej, domain.CodeExternalReferenceReferenceEmpty)
	if s := r.Snapshot(); s.SubjectID != "" {
		t.Fatalf("snapshot = %+v, want nothing linked", s)
	}
}

// A provider with a colon makes the key ambiguous: "p" with the reference
// "x:registry:asset:y" and "p:registry:asset:x" with "y" compose the same key,
// so the second is refused and the first, which holds the colon in its
// reference, keeps its link.
func TestLinkOfAProviderWithAColonIsRefusedBecauseItsKeyIsAmbiguous(t *testing.T) {
	first := link()
	first.ProviderID, first.Reference = "p", "x:registry:asset:y"
	second := link()
	second.ProviderID, second.Reference, second.SubjectID = "p:registry:asset:x", "y", "ast-2"
	key := domain.ReferenceKeyOf(first.ProviderID, first.System, first.ReferenceType, first.Reference)
	if other := domain.ReferenceKeyOf(second.ProviderID, second.System, second.ReferenceType, second.Reference); other != key {
		t.Fatalf("ReferenceKeyOf() = %q and %q, want the same key for both", key, other)
	}
	r := domain.NewExternalReference(key)

	_, rej := r.Link(first)
	tb.RequireAccepted(t, rej)
	before := r.Snapshot()

	acc, rej := r.Link(second)

	tb.RequireRejected(t, acc, rej, domain.CodeExternalReferenceProviderInvalid)
	if after := r.Snapshot(); !before.Equal(after) || after.ProviderID != "p" || after.Reference != "x:registry:asset:y" {
		t.Fatalf("snapshot = %+v, want the first link intact", after)
	}
}

func TestLinkOfAProviderWithAColonIsRefusedOnANewReference(t *testing.T) {
	cmd := link()
	cmd.ProviderID = "prt-1:production"
	r := domain.NewExternalReference(domain.ReferenceKeyOf(cmd.ProviderID, cmd.System, cmd.ReferenceType, cmd.Reference))

	acc, rej := r.Link(cmd)

	tb.RequireRejected(t, acc, rej, domain.CodeExternalReferenceProviderInvalid)
	if s := r.Snapshot(); s.SubjectID != "" {
		t.Fatalf("snapshot = %+v, want nothing linked", s)
	}
}
