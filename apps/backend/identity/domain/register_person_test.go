package domain_test

import (
	"fmt"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	personID     = domain.PersonID("per-100")
	fullName     = "Pessoa Sintetica da Silva"
	documentHash = "9b07f96fee3b329855d08503cfee6269d19587470dfa1a60c4c99cb463d6ec9e"
)

func registerPersonCommand() domain.RegisterPerson {
	return domain.RegisterPerson{
		FullName:               fullName,
		Document:               "123.456.789-09",
		DocumentHash:           documentHash,
		DocumentHashKeyVersion: 1,
		At:                     at,
	}
}

func registeredPerson() domain.PersonSnapshot {
	return domain.PersonSnapshot{
		ID:                     personID,
		FullName:               fullName,
		DocumentHash:           documentHash,
		DocumentHashKeyVersion: 1,
		DocumentMasked:         "***.***.*89-09",
		CreatedAt:              at,
	}
}

func TestRegisterPersonKeepsTheHashAndTheMaskWithoutTheDocument(t *testing.T) {
	p := domain.NewPerson(personID)

	acc, rej := p.Register(registerPersonCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisterPersonResponse{PersonID: personID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if got := p.Snapshot(); !got.Equal(registeredPerson()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, registeredPerson())
	}
}

func TestRegisterPersonEmitsNoEvent(t *testing.T) {
	acc, rej := domain.NewPerson(personID).Register(registerPersonCommand())

	tb.RequireAccepted(t, rej)
	if events := acc.Events(); len(events) != 0 {
		t.Fatalf("Events() = %v, want none", events)
	}
}

// The document is PII_SENSITIVE and is not stored in this version (spec note
// ‡‡): neither the value as received nor its digits may reach the state.
func TestRegisterPersonKeepsTheDocumentOutOfTheState(t *testing.T) {
	for _, raw := range []string{"123.456.789-09", syntheticCPF} {
		cmd := registerPersonCommand()
		cmd.Document = raw
		p := domain.NewPerson(personID)
		_, rej := p.Register(cmd)
		tb.RequireAccepted(t, rej)

		printed := fmt.Sprintf("%+v %#v", p.Snapshot(), p)
		for _, leaked := range []string{raw, domain.NormalizeDocument(raw)} {
			if strings.Contains(printed, leaked) {
				t.Fatalf("person %s carries the document %q", printed, leaked)
			}
		}
	}
}

func TestRegisterPersonRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.RegisterPerson)
		code   kernel.Code
	}{
		{"full name empty", func(c *domain.RegisterPerson) { c.FullName = "" }, domain.CodePersonFullNameEmpty},
		{"full name of blanks", func(c *domain.RegisterPerson) { c.FullName = " \t " }, domain.CodePersonFullNameEmpty},
		{"document empty", func(c *domain.RegisterPerson) { c.Document = "" }, domain.CodePersonDocumentEmpty},
		{"document of blanks", func(c *domain.RegisterPerson) { c.Document = "   " }, domain.CodePersonDocumentEmpty},
		{"document without a digit", func(c *domain.RegisterPerson) { c.Document = "abc.-/" }, domain.CodePersonDocumentEmpty},
		{"document already registered", func(c *domain.RegisterPerson) { c.DocumentAlreadyRegistered = true }, domain.CodePersonDuplicateDocument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registerPersonCommand()
			tt.mutate(&cmd)
			p := domain.NewPerson(personID)
			before := p.Snapshot()

			acc, rej := p.Register(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if len(rej.Details()) != 0 {
				t.Fatalf("Details() = %v, want none: a detail would carry personal data", rej.Details())
			}
			if after := p.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestRegisterPersonReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.RegisterPerson{DocumentAlreadyRegistered: true, At: at}
	want := []kernel.Code{
		domain.CodePersonFullNameEmpty,
		domain.CodePersonDocumentEmpty,
		domain.CodePersonDuplicateDocument,
	}
	fixes := []func(*domain.RegisterPerson){
		func(c *domain.RegisterPerson) { c.FullName = fullName },
		func(c *domain.RegisterPerson) { c.Document = syntheticCPF },
		func(*domain.RegisterPerson) {},
	}
	for i, code := range want {
		acc, rej := domain.NewPerson(personID).Register(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}

func TestRegisterPersonRejectionsCarryNoDocument(t *testing.T) {
	cmd := registerPersonCommand()
	cmd.DocumentAlreadyRegistered = true

	_, rej := domain.NewPerson(personID).Register(cmd)

	printed := fmt.Sprintf("%s %v", rej.Error(), rej.Details())
	if strings.Contains(printed, syntheticCPF) || strings.Contains(printed, "123.456.789-09") || strings.Contains(printed, documentHash) {
		t.Fatalf("rejection %q carries the document or its hash", printed)
	}
}

func TestRegisterPersonDeterminism(t *testing.T) {
	p1 := domain.NewPerson(personID)
	acc1, _ := p1.Register(registerPersonCommand())

	p2 := domain.NewPerson(personID)
	acc2, _ := p2.Register(registerPersonCommand())

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestFromPersonSnapshotRoundTrips(t *testing.T) {
	before := registeredPerson()

	got := domain.FromPersonSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestPersonEqualTellsEveryFieldApart(t *testing.T) {
	base := registeredPerson()
	for name, mutate := range map[string]func(*domain.PersonSnapshot){
		"id":              func(s *domain.PersonSnapshot) { s.ID = "per-other" },
		"full name":       func(s *domain.PersonSnapshot) { s.FullName = "other" },
		"document hash":   func(s *domain.PersonSnapshot) { s.DocumentHash = "other" },
		"key version":     func(s *domain.PersonSnapshot) { s.DocumentHashKeyVersion = 2 },
		"document masked": func(s *domain.PersonSnapshot) { s.DocumentMasked = "other" },
		"created at":      func(s *domain.PersonSnapshot) { s.CreatedAt++ },
	} {
		changed := base
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}
