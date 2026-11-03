package app

import (
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// dispatchSettle bounds the record of an outcome past the delivery's deadline.
const dispatchSettle = 5 * time.Second

// PartnerRoute is the hop of one submission to a partner. No adapter exists
// until the providers are mapped (#5), so it is the platform's deadline.
func PartnerRoute() deadline.Budget {
	return deadline.Budget{
		Dependency: "partner", Method: "submit", Limit: resilience.DefaultDeadline, Slack: deadline.DefaultSlack,
	}
}

// DispatchBudget is the budget of one submission: the route also covers the two
// writes of the raw store around the call (decision 17), so the sweep of
// attempts never abandons a submission still within its deadlines.
func DispatchBudget(partner deadline.Budget) (kernelapp.ExternalBudget, error) {
	put, _ := RawStoreSheet().Deadline.Get()
	route := partner
	route.Limit += 2 * put
	budget := kernelapp.ExternalBudget{Route: route, Settle: dispatchSettle, Margin: kernelapp.MinExternalMargin}
	budget.Processing = budget.StaleAfter()
	return budget, budget.Validate()
}

// DispatchBudgetOf is what the application reads of the budget.
func DispatchBudgetOf(budget kernelapp.ExternalBudget) application.DispatchBudget {
	return application.DispatchBudget{StaleAfter: int64(budget.StaleAfter()), SettleContext: budget.SettleContext()}
}
