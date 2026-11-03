package app

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
)

var partnerRegistered = channel.Event{
	Type:        "com.company.partner.partner-registered",
	ContractRef: "company/partner/event/v1/partner_registered.proto#PartnerRegistered",
}

// PartnerChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func PartnerChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group, partnerRegistered)
}
