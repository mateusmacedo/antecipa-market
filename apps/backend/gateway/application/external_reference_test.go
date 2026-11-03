package application_test

import (
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func registryAsset(subject domain.SubjectID) application.LinkExternalReference {
	return application.LinkExternalReference{
		ProviderID: "prt-1", System: domain.SystemRegistry, ReferenceType: domain.ReferenceAsset, Reference: " dup-0001 ",
		SubjectType: "asset", SubjectID: subject, AsOf: testOccurred, ObservedVersion: "v1",
	}
}

var registryAssetKey = domain.ReferenceKeyOf("prt-1", domain.SystemRegistry, domain.ReferenceAsset, " dup-0001 ")

func TestLinkExternalReferenceKeepsTheReferenceAsReceived(t *testing.T) {
	h := newHarness(t)

	got := must(h.service.LinkExternalReference(withExecution(t), registryAsset("ast-1"))).accepted(t)

	if got.ReferenceKey != registryAssetKey {
		t.Fatalf("Response() = %+v, want %q", got, registryAssetKey)
	}
	found, err := h.service.FindExternalReference(withExecution(t), registryAssetKey)
	if err != nil || found.Reference != " dup-0001 " || found.NormalizedReference != "DUP-0001" || found.SubjectID != "ast-1" {
		t.Fatalf("FindExternalReference() = %+v, %v; want the reference as received beside its normalized form", found, err)
	}
	if n := len(h.entries()); n != 0 {
		t.Fatalf("%d outbox entries, want none: a link emits no event", n)
	}
}

func TestLinkExternalReferenceToAnotherObjectIsRefused(t *testing.T) {
	h := newHarness(t)
	must(h.service.LinkExternalReference(withExecution(t), registryAsset("ast-1"))).accepted(t)

	must(h.service.LinkExternalReference(withExecution(t), registryAsset("ast-2"))).refused(t, string(domain.CodeExternalReferenceAlreadyLinked))
}

func TestLinkExternalReferenceWithAnEmptyReferenceIsRefused(t *testing.T) {
	h := newHarness(t)
	cmd := registryAsset("ast-1")
	cmd.Reference = ""

	must(h.service.LinkExternalReference(withExecution(t), cmd)).refused(t, string(domain.CodeExternalReferenceReferenceEmpty))
}

func TestLinkExternalReferenceRenewsTheObservationOfTheSameObject(t *testing.T) {
	h := newHarness(t)
	must(h.service.LinkExternalReference(withExecution(t), registryAsset("ast-1"))).accepted(t)
	newer := registryAsset("ast-1")
	newer.AsOf, newer.ObservedVersion = testOccurred+second, "v2"

	must(h.service.LinkExternalReference(withExecution(t), newer)).accepted(t)

	found, err := h.service.FindExternalReference(withExecution(t), registryAssetKey)
	if err != nil || found.AsOf != testOccurred+second || found.ObservedVersion != "v2" {
		t.Fatalf("FindExternalReference() = %+v, %v; want asOf and observedVersion renewed", found, err)
	}
}

// The reference key holds colons, which IDM-02 refuses, so the key of the
// link a normalization finds is derived under the digest.
func TestALinkFoundInAnInteractionReplaysUnderAKeyDerivedFromIt(t *testing.T) {
	h := newHarness(t)
	cmd := application.LinkInteractionReference{Link: registryAsset("ast-1"), InteractionID: testInteraction}
	must(h.service.LinkInteractionReference(withExecution(t), cmd)).accepted(t)

	other := cmd
	other.Link.ObservedVersion = "v9"
	if _, err := h.service.LinkInteractionReference(withExecution(t), other); !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("LinkInteractionReference(another link, same interaction) = %v, want ErrIdempotencyMismatch under the derived key", err)
	}
	must(h.service.LinkInteractionReference(withExecution(t), cmd)).accepted(t)
}

func TestFindExternalReferencesBySubjectNarrowsToTheSubjectType(t *testing.T) {
	h := newHarness(t)
	must(h.service.LinkExternalReference(withExecution(t), registryAsset("obj-1"))).accepted(t)
	effect := registryAsset("obj-1")
	effect.ReferenceType, effect.Reference, effect.SubjectType = domain.ReferenceEffect, "eff-77", "effect"
	must(h.service.LinkExternalReference(withExecution(t), effect)).accepted(t)

	found, err := h.service.FindExternalReferencesBySubject(withExecution(t), "asset", "obj-1")

	if err != nil || len(found) != 1 || found[0].Key != registryAssetKey {
		t.Fatalf("FindExternalReferencesBySubject(asset, obj-1) = %+v, %v; want only the asset reference", found, err)
	}
	none, err := h.service.FindExternalReferencesBySubject(withExecution(t), "asset", "obj-9")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("FindExternalReferencesBySubject(asset, obj-9) = %#v, %v; want an empty list", none, err)
	}
}

func TestFindExternalInteractionByPartnerEventAnswersTheFirstReception(t *testing.T) {
	h := newHarness(t)
	h.recorded(t, inboundCallback())
	repeated := inboundCallback()
	repeated.InteractionID, repeated.DuplicateOf = "int-2", testInteraction
	h.recorded(t, repeated)

	found, err := h.service.FindExternalInteractionByPartnerEvent(withExecution(t), testConnector, "evt-p-1")
	if err != nil || found.ID != testInteraction {
		t.Fatalf("FindExternalInteractionByPartnerEvent() = %+v, %v; want the first reception", found, err)
	}
	if _, err := h.service.FindExternalInteractionByPartnerEvent(withExecution(t), "prt-2:production", "evt-p-1"); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindExternalInteractionByPartnerEvent(another connector) = %v, want ErrNotFound", err)
	}
}

func TestFindExternalInteractionsByCommandListsTheInteractionsOfTheCommand(t *testing.T) {
	h := newHarness(t)
	capture := inboundCallback()
	capture.CommandID = testCommand
	h.recorded(t, capture)
	other := inboundCallback()
	other.InteractionID, other.PartnerEventID = "int-2", "evt-p-2"
	h.recorded(t, other)

	found, err := h.service.FindExternalInteractionsByCommand(withExecution(t), testCommand)
	if err != nil || len(found) != 1 || found[0].ID != testInteraction {
		t.Fatalf("FindExternalInteractionsByCommand() = %+v, %v; want the interaction of the command", found, err)
	}
	one, err := h.service.FindExternalInteraction(withExecution(t), "int-2")
	if err != nil || one.PartnerEventID != "evt-p-2" {
		t.Fatalf("FindExternalInteraction(int-2) = %+v, %v", one, err)
	}
}

// The key of a provider with a colon is ambiguous: it is refused, and the
// link it would have reached keeps the reference with the colon it holds.
func TestLinkExternalReferenceOfAProviderWithAColonIsRefused(t *testing.T) {
	h := newHarness(t)
	first := registryAsset("ast-1")
	first.ProviderID, first.Reference = "p", "x:registry:asset:y"
	must(h.service.LinkExternalReference(withExecution(t), first)).accepted(t)
	second := registryAsset("ast-2")
	second.ProviderID, second.Reference = "p:registry:asset:x", "y"

	must(h.service.LinkExternalReference(withExecution(t), second)).refused(t, string(domain.CodeExternalReferenceProviderInvalid))

	found, err := h.service.FindExternalReference(withExecution(t), "p:registry:asset:x:registry:asset:y")
	if err != nil || found.ProviderID != "p" || found.Reference != "x:registry:asset:y" || found.SubjectID != "ast-1" {
		t.Fatalf("FindExternalReference() = %+v, %v; want the first link intact", found, err)
	}
}
