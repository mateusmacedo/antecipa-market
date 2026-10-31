package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (c *Connector) SuspendOperation(cmd SuspendOperation) (kernel.Accepted[ConnectorResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Connector).clone, func(next *Connector) (kernel.Accepted[ConnectorResponse], *kernel.Rejection) {
		response := ConnectorResponse{ConnectorKey: next.key}
		// WHY: the Partner keys its events by capability, so versions of one
		// series may cross; an older one must not undo what a newer one set.
		if cmd.Cause == CausePartner && cmd.Version < next.appliedVersion(cmd.Operation, cmd.AssetType) {
			return kernel.Accept(response), nil
		}
		repeated := slices.ContainsFunc(next.suspensions, func(s OperationSuspension) bool {
			return s.Operation == cmd.Operation && s.AssetType == cmd.AssetType && s.Cause == cmd.Cause
		})
		if !repeated {
			next.suspensions = append(next.suspensions, OperationSuspension{
				Operation:    cmd.Operation,
				AssetType:    cmd.AssetType,
				Cause:        cmd.Cause,
				CapabilityID: cmd.CapabilityID,
				Version:      cmd.Version,
				Since:        cmd.At,
			})
		}
		return kernel.Accept(response), nil
	})
}
