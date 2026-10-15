package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (p *Person) Register(cmd RegisterPerson) (kernel.Accepted[RegisterPersonResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*Person).clone, func(next *Person) (kernel.Accepted[RegisterPersonResponse], *kernel.Rejection) {
		switch {
		case strings.TrimSpace(cmd.FullName) == "":
			return kernel.Refuse[RegisterPersonResponse](CodePersonFullNameEmpty, "full name must not be empty")
		case NormalizeDocument(cmd.Document) == "":
			return kernel.Refuse[RegisterPersonResponse](CodePersonDocumentEmpty, "document must not be empty")
		case cmd.DocumentAlreadyRegistered:
			return kernel.Refuse[RegisterPersonResponse](CodePersonDuplicateDocument, "document already registered")
		}
		next.fullName = cmd.FullName
		next.documentHash = cmd.DocumentHash
		next.documentHashKeyVersion = cmd.DocumentHashKeyVersion
		next.documentMasked = MaskDocument(cmd.Document)
		next.createdAt = cmd.At
		return kernel.Accept(RegisterPersonResponse{PersonID: next.id}), nil
	})
}
