package domain_test

import (
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// The eleven reads of the ports (04 §3) are what ReadExternal serves; only the
// four of ReceivablesQueryPort answer to a QueryAuthorization.
func TestTheReadVocabularyIsTheElevenReadsOfThePorts(t *testing.T) {
	guarded := []string{"discoverSchedule", "queryAsset", "queryOwnership", "queryEncumbrances"}
	unguarded := []string{
		"queryOptInStatus", "listAuthorizations", "queryEffect", "queryEffectsByAsset",
		"querySettlements", "querySettlementByAsset", "fetchEvidence",
	}
	for _, operation := range guarded {
		if !domain.IsReadOperation(operation) || !domain.RequiresQueryAuthorization(operation) {
			t.Errorf("%s: want a read guarded by the authorization of query", operation)
		}
	}
	for _, operation := range unguarded {
		if !domain.IsReadOperation(operation) || domain.RequiresQueryAuthorization(operation) {
			t.Errorf("%s: want a read guarded by the capability alone", operation)
		}
	}
	for _, operation := range []string{"submitEffect", "activateOptIn", "storeInteraction", "QueryAsset", ""} {
		if domain.IsReadOperation(operation) || domain.RequiresQueryAuthorization(operation) {
			t.Errorf("%q: want no read of the ports", operation)
		}
	}
}
