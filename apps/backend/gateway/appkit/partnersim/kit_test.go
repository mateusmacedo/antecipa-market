package partnersim_test

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
)

// The simulator's adapter is certified like a real one would be (KIT-04): the
// table it reads its responses by is the contract with the partner.
func TestTheSimulatorAdapterConformsToTheExternalCommandKit(t *testing.T) {
	tb.Require(t, providerkit.ExternalCommand(func() providerkit.ExternalCommandSubject {
		sim := newFixture(t, nil).sim
		return providerkit.ExternalCommandSubject{Table: sim.Table(), Classify: sim.Classify, Validate: partnersim.ValidateTable}
	}))
}

func TestTheSimulatorRefusesATableMappingAStatusToNoClass(t *testing.T) {
	table := partnersim.DefaultTable()
	table[299] = port.ClassSchemaMismatch + 1

	if _, err := partnersim.New(memory.FixedClock{At: at}, &memory.SequenceIDs{}, nil, partnersim.WithTable(table)); !errors.Is(err, partnersim.ErrInvalidTable) {
		t.Fatalf("New(invalid table) = %v, want ErrInvalidTable", err)
	}
}
