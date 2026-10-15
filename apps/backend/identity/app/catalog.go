package app

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
)

const (
	organizationRegisteredEventType = "com.company.identity.organization-registered"
	organizationRegisteredContract  = "company/identity/event/v1/organization_registered.proto#OrganizationRegistered"
)

// IdentityChannel is the channel the relay publishes to, named after the
// destination the use case authors, because the publisher resolves by it.
func IdentityChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.IdentityTopic, cfg.IdentityDLQ, cfg.Group,
		organizationRegisteredEventType, organizationRegisteredContract)
}
