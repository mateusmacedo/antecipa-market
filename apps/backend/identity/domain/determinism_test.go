package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestRegisterDeterminism(t *testing.T) {
	o1 := domain.NewOrganization(organizationID)
	acc1, _ := o1.Register(registerCommand())

	o2 := domain.NewOrganization(organizationID)
	acc2, _ := o2.Register(registerCommand())

	if !o1.Snapshot().Equal(o2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}
