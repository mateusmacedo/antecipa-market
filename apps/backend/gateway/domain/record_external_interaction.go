package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Record keeps the raw payload before any deduplication or reading (04 §7,
// steps 2 and 3): a duplicate is kept too, but only a stored interaction goes
// to the normalization.
func (i *ExternalInteraction) Record(cmd RecordExternalInteraction) (kernel.Accepted[InteractionResponse], *kernel.Rejection) {
	return kernel.DecideOver(i, (*ExternalInteraction).clone, func(next *ExternalInteraction) (kernel.Accepted[InteractionResponse], *kernel.Rejection) {
		if !cmd.Request.stored() && !cmd.Response.stored() {
			return kernel.Refuse[InteractionResponse](CodeExternalInteractionRawPayloadMissing,
				"an interaction is recorded only with the reference of its raw payload")
		}
		next.connectorKey = cmd.ConnectorKey
		next.operation = cmd.Operation
		next.direction = cmd.Direction
		next.source = cmd.Source
		next.commandID = cmd.CommandID
		next.contentType = cmd.ContentType
		next.capturedStep = cmd.CapturedStep
		next.partnerEventID = cmd.PartnerEventID
		next.businessDate = cmd.BusinessDate
		next.request = cmd.Request
		next.response = cmd.Response
		next.mismatch = cmd.Mismatch
		next.duplicateOf = cmd.DuplicateOf
		next.normalizationRevision = 1
		next.receivedAt = cmd.At
		response := InteractionResponse{InteractionID: next.id}
		if cmd.DuplicateOf != "" {
			next.status = InteractionDuplicate
			response.Status = next.status
			return kernel.Accept(response), nil
		}
		next.status = InteractionStored
		response.Status = next.status
		return kernel.Accept(response, next.stored(cmd.At)), nil
	})
}
