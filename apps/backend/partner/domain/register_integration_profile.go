package domain

import (
	"slices"
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (p *IntegrationProfile) Register(cmd RegisterIntegrationProfile) (kernel.Accepted[RegisterIntegrationProfileResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*IntegrationProfile).clone, func(next *IntegrationProfile) (kernel.Accepted[RegisterIntegrationProfileResponse], *kernel.Rejection) {
		switch {
		case strings.TrimSpace(cmd.Endpoint) == "":
			return kernel.Refuse[RegisterIntegrationProfileResponse](CodeIntegrationProfileEndpointEmpty, "the endpoint is required")
		case strings.TrimSpace(cmd.SecretRef) == "":
			return kernel.Refuse[RegisterIntegrationProfileResponse](CodeIntegrationProfileSecretRefMissing, "the reference to the secret is required")
		case cmd.CertificateExpiresAt <= cmd.At:
			return kernel.Refuse[RegisterIntegrationProfileResponse](CodeIntegrationProfileCertificateExpiryInvalid, "the certificate must expire after the registration")
		case cmd.SupersedesID != cmd.Head.ID:
			return kernel.Refuse[RegisterIntegrationProfileResponse](CodeIntegrationProfileSupersedesInvalid,
				"supersedes must name the last version of the series of the partner and environment, and a series holds one first version",
				kernel.Detail{Key: "supersedes", Value: string(cmd.SupersedesID)},
			)
		}
		next.partnerID = cmd.PartnerID
		next.environment = cmd.Environment
		next.version = cmd.Head.Version + 1
		next.supersedesID = cmd.SupersedesID
		next.changeKind = cmd.ChangeKind
		next.protocols = slices.Clone(cmd.Protocols)
		next.endpoint = cmd.Endpoint
		next.schemaVersion = cmd.SchemaVersion
		next.secretRef = cmd.SecretRef
		next.certificateRef = cmd.CertificateRef
		next.certificateExpiresAt = cmd.CertificateExpiresAt
		next.effectiveFrom = cmd.EffectiveFrom
		return kernel.Accept(RegisterIntegrationProfileResponse{ProfileID: next.id}), nil
	})
}
