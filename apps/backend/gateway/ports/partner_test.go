package ports_test

import (
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

const connector = domain.ConnectorKey("prt-1:production")

func TestTheRawKeyNamesTheConnectorTheInteractionAndTheSide(t *testing.T) {
	for part, want := range map[ports.RawPart]string{
		ports.RawRequest:  "interactions/prt-1:production/int-1/request",
		ports.RawResponse: "interactions/prt-1:production/int-1/response",
	} {
		if got := ports.RawKeyOf(connector, "int-1", part); got != want {
			t.Fatalf("RawKeyOf(%s) = %q, want %q", part, got, want)
		}
		if err := port.ObjectKey(ports.RawKeyOf(connector, "int-1", part)).Validate(); err != nil {
			t.Fatalf("ObjectKey(%s).Validate() = %v, want a key the store accepts", part, err)
		}
	}
}

// Until the providers are mapped the runtime has no adapter, and every way to
// a partner fails closed before any call.
func TestWithoutAnAdapterEveryWayToThePartnerFailsClosed(t *testing.T) {
	var adapters ports.PartnerAdapters = ports.NoPartnerAdapters{}

	command, err := adapters.Command(connector, "submitEffect", &ports.CaptureCollector{})
	if command != nil || !errors.Is(err, ports.ErrPartnerAdapterUnavailable) {
		t.Fatalf("Command() = %v, %v; want ErrPartnerAdapterUnavailable", command, err)
	}
	reader, err := adapters.Reader(connector, "queryAsset")
	if reader != nil || !errors.Is(err, ports.ErrPartnerAdapterUnavailable) {
		t.Fatalf("Reader() = %v, %v; want ErrPartnerAdapterUnavailable", reader, err)
	}
	normalizer, err := adapters.Normalizer(connector, "submitEffect")
	if normalizer != nil || !errors.Is(err, ports.ErrPartnerAdapterUnavailable) {
		t.Fatalf("Normalizer() = %v, %v; want ErrPartnerAdapterUnavailable", normalizer, err)
	}
}

func TestTheCollectorHandsOverACaptureOnlyWhenBothSidesWereKept(t *testing.T) {
	request := domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "v1", Hash: "h1"}
	response := domain.RawPayload{Ref: "interactions/prt-1:production/int-1/response", VersionID: "v2", Hash: "h2"}
	broken := domain.Mismatch{Kind: domain.MismatchVersion, Detail: "format", PartnerDeclaredVersion: "v9"}
	var c ports.CaptureCollector

	c.Begin("int-1")
	c.Requested(request)
	if _, ok := c.Captured(); ok {
		t.Fatal("Captured() = ok with only the request kept, want no capture")
	}
	c.Responded(response, "application/json")
	c.Broke(broken)
	got, ok := c.Captured()
	if want := (ports.Capture{InteractionID: "int-1", Request: request, Response: response, ContentType: "application/json", Mismatch: broken}); !ok || got != want {
		t.Fatalf("Captured() = %+v, %v; want %+v", got, ok, want)
	}

	lost := errors.New("store down")
	c.Begin("int-2")
	c.Requested(request)
	c.ResponseLost(lost)
	if _, ok := c.Captured(); ok || !errors.Is(c.LostResponse(), lost) {
		t.Fatalf("after a lost response Captured() ok = %v, LostResponse() = %v; want no capture and the failure", ok, c.LostResponse())
	}

	c.Begin("int-3")
	if _, ok := c.Captured(); ok || c.LostResponse() != nil {
		t.Fatal("Begin kept the previous call, want every call to start over")
	}
}
