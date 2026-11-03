package partnersim

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrInvalidTable is a partner table mapping a status to no class.
var ErrInvalidTable = errors.New("partnersim: the partner table maps a status to no class")

// DefaultTable is the status the simulated partner answers with and the class
// each one lands on, the table KIT-04 certifies.
func DefaultTable() map[int]port.ExternalClass {
	return map[int]port.ExternalClass{
		200: port.ClassAccepted,
		201: port.ClassAccepted,
		202: port.ClassAccepted,
		400: port.ClassFinalValidation,
		401: port.ClassFinalAuthorization,
		403: port.ClassFinalAuthorization,
		422: port.ClassFinalBusiness,
		429: port.ClassRetryableUpstream,
		502: port.ClassRetryableUpstream,
		503: port.ClassRetryableUpstream,
		504: port.ClassUnknownSent,
	}
}

func ValidateTable(table map[int]port.ExternalClass) error {
	for _, status := range slices.Sorted(maps.Keys(table)) {
		if class := table[status]; class < port.ClassAccepted || class > port.ClassSchemaMismatch {
			return fmt.Errorf("%w: status %d maps to %d", ErrInvalidTable, status, class)
		}
	}
	return nil
}

// classify lands a status the partner never declared on unknown_ambiguous:
// it is no evidence of what the partner did.
func classify(table map[int]port.ExternalClass, status int) port.ExternalClass {
	if class, ok := table[status]; ok {
		return class
	}
	return port.ClassUnknownAmbiguous
}
