package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Create makes the command durable before anything is sent: the dispatch
// intent goes to the outbox with the state, and the handler sends it (RES-45).
func (c *ExternalCommand) Create(cmd CreateExternalCommand) (kernel.Accepted[CommandStatusResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*ExternalCommand).clone, func(next *ExternalCommand) (kernel.Accepted[CommandStatusResponse], *kernel.Rejection) {
		// WHY: a taken key answers for the command it names; a gate closed since
		// would refuse a repeat as if the command had never been created.
		if next.createdAt != 0 {
			if cmd.PayloadHash != next.payloadHash {
				return kernel.Refuse[CommandStatusResponse](CodeExternalCommandIdempotencyKeyConflict,
					"the idempotency key already names a command with another payload")
			}
			return kernel.Accept(CommandStatusResponse{IdempotencyKey: next.key, Status: next.status}), nil
		}
		switch cmd.CapabilityGate {
		case GateAllowed:
		case GateSuspended:
			return kernel.Refuse[CommandStatusResponse](CodeExternalCommandOperationSuspended, "the operation is suspended for this connector",
				kernel.Detail{Key: "operation", Value: cmd.Operation})
		default:
			return kernel.Refuse[CommandStatusResponse](CodeExternalCommandCapabilityNotEnabled, "the operation is not enabled for this connector",
				kernel.Detail{Key: "operation", Value: cmd.Operation}, kernel.Detail{Key: "assetType", Value: cmd.AssetType})
		}
		if cmd.BusinessKey == "" {
			return kernel.Refuse[CommandStatusResponse](CodeExternalCommandBusinessKeyEmpty, "businessKey must not be empty")
		}
		if cmd.Payload == "" {
			return kernel.Refuse[CommandStatusResponse](CodeExternalCommandPayloadEmpty, "payload must not be empty")
		}
		if !cmd.SeriesOpen {
			return kernel.Refuse[CommandStatusResponse](CodeExternalCommandContentChangedAfterSend,
				"another version of the series is still alive: cancel it if unsent, or revise it, never resend")
		}
		next.tenantID = cmd.TenantID
		next.connectorKey = cmd.ConnectorKey
		next.operation = cmd.Operation
		next.assetType = cmd.AssetType
		next.businessKey = cmd.BusinessKey
		next.payloadVersion = cmd.PayloadVersion
		next.seriesKey = SeriesKeyOf(cmd.ConnectorKey, cmd.Operation, cmd.BusinessKey)
		next.payload = cmd.Payload
		next.payloadHash = cmd.PayloadHash
		next.profile = cmd.Profile
		next.outboundKey = cmd.OutboundKey
		next.status = StatusCreated
		next.createdAt = cmd.At
		next.submissions = 0
		next.reconciliationRequired = false
		return kernel.Accept(CommandStatusResponse{IdempotencyKey: next.key, Status: next.status}, next.dispatchRequested(cmd.At)), nil
	})
}
