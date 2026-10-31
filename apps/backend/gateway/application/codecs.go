package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

var connectorCodec = usecase.OutcomeCodec[domain.ConnectorResponse]{
	Encode: func(e *usecase.Encoder, r domain.ConnectorResponse) { e.String(string(r.ConnectorKey)) },
	Decode: func(d *usecase.Decoder) domain.ConnectorResponse {
		return domain.ConnectorResponse{ConnectorKey: domain.ConnectorKey(d.String())}
	},
}

var profileCodec = usecase.OutcomeCodec[domain.ProfileResponse]{
	Encode: func(e *usecase.Encoder, r domain.ProfileResponse) { e.String(string(r.ProfileKey)) },
	Decode: func(d *usecase.Decoder) domain.ProfileResponse {
		return domain.ProfileResponse{ProfileKey: domain.ProfileKey(d.String())}
	},
}

var authorizationCodec = usecase.OutcomeCodec[domain.AuthorizationResponse]{
	Encode: func(e *usecase.Encoder, r domain.AuthorizationResponse) { e.String(string(r.ConsentID)) },
	Decode: func(d *usecase.Decoder) domain.AuthorizationResponse {
		return domain.AuthorizationResponse{ConsentID: domain.ConsentID(d.String())}
	},
}

var commandCodec = usecase.OutcomeCodec[domain.CommandResponse]{
	Encode: func(e *usecase.Encoder, r domain.CommandResponse) { e.String(string(r.IdempotencyKey)) },
	Decode: func(d *usecase.Decoder) domain.CommandResponse {
		return domain.CommandResponse{IdempotencyKey: domain.IdempotencyKey(d.String())}
	},
}

var commandStatusCodec = usecase.OutcomeCodec[domain.CommandStatusResponse]{
	Encode: func(e *usecase.Encoder, r domain.CommandStatusResponse) {
		e.String(string(r.IdempotencyKey))
		e.String(string(r.Status))
	},
	Decode: func(d *usecase.Decoder) domain.CommandStatusResponse {
		key := domain.IdempotencyKey(d.String())
		return domain.CommandStatusResponse{IdempotencyKey: key, Status: domain.CommandStatus(d.String())}
	},
}
