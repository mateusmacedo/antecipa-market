package domain

import (
	"slices"
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

const day kernel.Instant = 86_400_000_000_000

var certificateAlertThresholds = []int{60, 30, 7}

func (p *IntegrationProfile) RaiseExpiryAlert(cmd RaiseCertificateExpiryAlert) (kernel.Accepted[RaiseCertificateExpiryAlertResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*IntegrationProfile).clone, func(next *IntegrationProfile) (kernel.Accepted[RaiseCertificateExpiryAlertResponse], *kernel.Rejection) {
		switch {
		case !slices.Contains(certificateAlertThresholds, cmd.ThresholdDays):
			return kernel.Refuse[RaiseCertificateExpiryAlertResponse](CodeIntegrationProfileInvalidAlertThreshold,
				"the alert threshold is 60, 30 or 7 days",
				kernel.Detail{Key: "thresholdDays", Value: strconv.Itoa(cmd.ThresholdDays)},
			)
		case next.certificateExpiresAt-kernel.Instant(cmd.ThresholdDays)*day > cmd.At:
			return kernel.Refuse[RaiseCertificateExpiryAlertResponse](CodeIntegrationProfileThresholdNotReached, "the certificate is not yet within the alert threshold")
		case next.alertedWithin(cmd.ThresholdDays):
			return kernel.Refuse[RaiseCertificateExpiryAlertResponse](CodeIntegrationProfileAlreadyAlerted, "this threshold or a smaller one was already alerted")
		}
		next.lastAlertedThresholdDays = cmd.ThresholdDays
		return kernel.Accept(
			RaiseCertificateExpiryAlertResponse{ProfileID: next.id},
			PartnerCertificateExpiring{
				ProfileID:            next.id,
				PartnerID:            next.partnerID,
				Environment:          next.environment,
				CertificateExpiresAt: next.certificateExpiresAt,
				ThresholdDays:        cmd.ThresholdDays,
				At:                   cmd.At,
			},
		), nil
	})
}

func (p *IntegrationProfile) alertedWithin(thresholdDays int) bool {
	return p.lastAlertedThresholdDays != 0 && p.lastAlertedThresholdDays <= thresholdDays
}
