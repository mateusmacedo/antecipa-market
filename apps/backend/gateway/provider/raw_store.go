package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// RawStore realizes ports.RawStore over the kernel's object storage port. It
// holds only the writer and the reader: the context never retains, holds nor
// purges a version (decision 16), so it never receives those interfaces.
type RawStore struct {
	writer port.ObjectWriter
	reader port.ObjectReader
}

var _ ports.RawStore = RawStore{}

func NewRawStore(writer port.ObjectWriter, reader port.ObjectReader) RawStore {
	return RawStore{writer: writer, reader: reader}
}

// Put writes a new version with no retention; the port checks the body against
// the SHA-256 computed here before writing.
func (s RawStore) Put(ctx context.Context, key string, body ports.RawBody) (domain.RawPayload, error) {
	sum := sha256.Sum256(body.Body)
	version, err := s.writer.Put(ctx, port.PutRequest{
		Key: port.ObjectKey(key), Body: body.Body, ContentSHA256: hex.EncodeToString(sum[:]), ContentType: body.ContentType,
	})
	if err != nil {
		return domain.RawPayload{}, rawFailure("put", err)
	}
	return domain.RawPayload{Ref: string(version.Key), VersionID: string(version.Version), Hash: version.ContentSHA256}, nil
}

// Get reads the recorded version and refuses one whose content is not the
// hash the interaction recorded, besides the check the port makes on read.
func (s RawStore) Get(ctx context.Context, payload domain.RawPayload) (ports.RawBody, error) {
	object, err := s.reader.Get(ctx, port.ObjectRef{Key: port.ObjectKey(payload.Ref), Version: port.VersionID(payload.VersionID)})
	if err != nil {
		return ports.RawBody{}, rawFailure("get", err)
	}
	if payload.Hash != "" && object.ContentSHA256 != payload.Hash {
		return ports.RawBody{}, rawFailure("get", fmt.Errorf("%w: the version is not the recorded content", port.ErrIntegrity))
	}
	return ports.RawBody{ContentType: object.ContentType, Body: object.Body}, nil
}

// rawFailure keeps the store's error in the chain under the sentinel that
// tells the caller whether repeating can help.
func rawFailure(operation string, err error) error {
	sentinel := ports.ErrRawStoreUnavailable
	for _, defect := range []error{
		port.ErrIntegrity, port.ErrObjectKeyInvalid, port.ErrVersionRequired, port.ErrObjectNotFound,
		port.ErrScopeMismatch, port.ErrContextAbsent,
	} {
		if errors.Is(err, defect) {
			sentinel = ports.ErrRawPayloadDefect
			break
		}
	}
	return fmt.Errorf("provider: raw store %s: %w: %w", operation, sentinel, err)
}
