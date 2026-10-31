package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (c *Connector) Register(cmd RegisterConnector) (kernel.Accepted[ConnectorResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Connector).clone, func(next *Connector) (kernel.Accepted[ConnectorResponse], *kernel.Rejection) {
		if cmd.CredentialRef == "" {
			return kernel.Refuse[ConnectorResponse](CodeConnectorCredentialRefEmpty, "credentialRef must not be empty")
		}
		response := ConnectorResponse{ConnectorKey: next.key}
		switch {
		case next.registeredAt == 0:
			next.partnerID = cmd.PartnerID
			next.environment = cmd.Environment
			next.credentialRef = cmd.CredentialRef
			next.integrationVersion = cmd.IntegrationVersion
			next.capabilities = []EnabledCapability{}
			next.suspensions = []OperationSuspension{}
			next.registeredAt = cmd.At
		case cmd.IntegrationVersion > next.integrationVersion:
			next.credentialRef = cmd.CredentialRef
			next.integrationVersion = cmd.IntegrationVersion
		}
		return kernel.Accept(response), nil
	})
}
