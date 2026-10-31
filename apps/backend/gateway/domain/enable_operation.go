package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// EnableOperation also lifts a schema_mismatch suspension: the Partner enables
// again only after the re-homologation that the mismatch demanded (decision 2).
func (c *Connector) EnableOperation(cmd EnableOperation) (kernel.Accepted[ConnectorResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Connector).clone, func(next *Connector) (kernel.Accepted[ConnectorResponse], *kernel.Rejection) {
		response := ConnectorResponse{ConnectorKey: next.key}
		if cmd.Version < next.appliedVersion(cmd.Operation, cmd.AssetType) {
			return kernel.Accept(response), nil
		}
		next.capabilities = slices.DeleteFunc(next.capabilities, func(e EnabledCapability) bool {
			return e.Operation == cmd.Operation && e.AssetType == cmd.AssetType
		})
		next.capabilities = append(next.capabilities, EnabledCapability(cmd))
		next.suspensions = slices.DeleteFunc(next.suspensions, func(s OperationSuspension) bool {
			return s.Operation == cmd.Operation && (s.AssetType == cmd.AssetType || s.AssetType == "")
		})
		return kernel.Accept(response), nil
	})
}
