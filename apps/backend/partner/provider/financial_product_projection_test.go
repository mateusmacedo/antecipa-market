package provider_test

import (
	"strconv"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestFinancialProductsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "financial-product.golden")
	if f.Identity.Context != "partner" || f.Identity.Aggregate != "financial-product" || len(f.Cases) != 11 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runFinancialProduct)
}

func runFinancialProduct(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	before := productFromState(t, c.StateBefore)
	at := kernel.Instant(integer(t, c.Command["at"]))
	response := func(id domain.ProductID) domainkit.Fields { return domainkit.Fields{"product": string(id)} }
	switch c.Command["upr"] {
	case "define":
		s := productSubject(func(p *domain.FinancialProduct) (kernel.Accepted[domain.DefineProductResponse], *kernel.Rejection) {
			return p.Define(domain.DefineProduct{
				PartnerID:             domain.PartnerID(c.Command["partner"]),
				AgreementID:           domain.AgreementID(c.Command["agreement"]),
				Code:                  c.Command["code"],
				Name:                  c.Command["name"],
				AssetTypes:            listFrom(c.Command["asset_types"]),
				Regions:               listFrom(c.Command["regions"]),
				Currency:              c.Command["currency"],
				MinAmountMinor:        integer(t, c.Command["min_amount_minor"]),
				MaxAmountMinor:        integer(t, c.Command["max_amount_minor"]),
				MinTermDays:           int(integer(t, c.Command["min_term_days"])),
				MaxTermDays:           int(integer(t, c.Command["max_term_days"])),
				RequiredCheckIDs:      listFrom(c.Command["required_check_ids"]),
				ValidFrom:             kernel.Instant(integer(t, c.Command["valid_from"])),
				ValidUntil:            kernel.Instant(integer(t, c.Command["valid_until"])),
				SupersedesID:          domain.ProductID(c.Command["supersedes"]),
				Head:                  domain.ProductHead{ID: domain.ProductID(c.Command["head"]), Version: int(integer(t, c.Command["head_version"]))},
				HasEffectiveAgreement: boolean(t, c.Command["has_effective_agreement"]),
				At:                    at,
			})
		}, func(r domain.DefineProductResponse) domainkit.Fields { return response(r.ProductID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "retire":
		s := productSubject(func(p *domain.FinancialProduct) (kernel.Accepted[domain.RetireProductResponse], *kernel.Rejection) {
			return p.Retire(domain.RetireProduct{Reason: domain.ProductRetireReason(c.Command["reason"]), At: at})
		}, func(r domain.RetireProductResponse) domainkit.Fields { return response(r.ProductID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func productSubject[R any](decide func(*domain.FinancialProduct) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.FinancialProduct, R] {
	return domainkit.Subject[*domain.FinancialProduct, R]{
		Decide:   decide,
		Response: response,
		Event:    productEvent,
		Snapshot: productState,
		Clone: func(p *domain.FinancialProduct) *domain.FinancialProduct {
			return domain.FromFinancialProductSnapshot(p.Snapshot())
		},
	}
}

func productState(p *domain.FinancialProduct) domainkit.Fields {
	s := p.Snapshot()
	return domainkit.Fields{
		"product":            string(s.ID),
		"partner":            string(s.PartnerID),
		"agreement":          string(s.AgreementID),
		"code":               s.Code,
		"name":               s.Name,
		"version":            strconv.Itoa(s.Version),
		"supersedes":         string(s.SupersedesID),
		"status":             string(s.Status),
		"valid_from":         instant(s.ValidFrom),
		"valid_until":        instant(s.ValidUntil),
		"asset_types":        strings.Join(s.AssetTypes, ","),
		"regions":            strings.Join(s.Regions, ","),
		"currency":           s.Currency,
		"min_amount_minor":   strconv.FormatInt(s.MinAmountMinor, 10),
		"max_amount_minor":   strconv.FormatInt(s.MaxAmountMinor, 10),
		"min_term_days":      strconv.Itoa(s.MinTermDays),
		"max_term_days":      strconv.Itoa(s.MaxTermDays),
		"required_check_ids": strings.Join(s.RequiredCheckIDs, ","),
	}
}

func productFromState(t *testing.T, f domainkit.Fields) *domain.FinancialProduct {
	t.Helper()
	return domain.FromFinancialProductSnapshot(domain.FinancialProductSnapshot{
		ID:               domain.ProductID(f["product"]),
		PartnerID:        domain.PartnerID(f["partner"]),
		AgreementID:      domain.AgreementID(f["agreement"]),
		Code:             f["code"],
		Name:             f["name"],
		Version:          int(integer(t, f["version"])),
		SupersedesID:     domain.ProductID(f["supersedes"]),
		Status:           domain.ProductStatus(f["status"]),
		ValidFrom:        kernel.Instant(integer(t, f["valid_from"])),
		ValidUntil:       kernel.Instant(integer(t, f["valid_until"])),
		AssetTypes:       listFrom(f["asset_types"]),
		Regions:          listFrom(f["regions"]),
		Currency:         f["currency"],
		MinAmountMinor:   integer(t, f["min_amount_minor"]),
		MaxAmountMinor:   integer(t, f["max_amount_minor"]),
		MinTermDays:      int(integer(t, f["min_term_days"])),
		MaxTermDays:      int(integer(t, f["max_term_days"])),
		RequiredCheckIDs: listFrom(f["required_check_ids"]),
	})
}

func productEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch event := e.(type) {
	case domain.PartnerProductDefined:
		return event.EventName(), productDefinitionFields(event.ProductID, event.PartnerID, event.AgreementID, event.Code, event.Version,
			event.ValidFrom, event.ValidUntil, event.AssetTypes, event.Regions, event.Currency,
			event.MinAmountMinor, event.MaxAmountMinor, event.MinTermDays, event.MaxTermDays, event.At)
	case domain.PartnerProductChanged:
		fields := productDefinitionFields(event.ProductID, event.PartnerID, event.AgreementID, event.Code, event.Version,
			event.ValidFrom, event.ValidUntil, event.AssetTypes, event.Regions, event.Currency,
			event.MinAmountMinor, event.MaxAmountMinor, event.MinTermDays, event.MaxTermDays, event.At)
		fields["supersedes"] = string(event.SupersedesID)
		return event.EventName(), fields
	case domain.PartnerProductExpired:
		return event.EventName(), domainkit.Fields{
			"product":     string(event.ProductID),
			"partner":     string(event.PartnerID),
			"version":     strconv.Itoa(event.Version),
			"valid_until": instant(event.ValidUntil),
			"at":          instant(event.At),
		}
	case domain.PartnerProductRetired:
		return event.EventName(), domainkit.Fields{
			"product": string(event.ProductID),
			"partner": string(event.PartnerID),
			"version": strconv.Itoa(event.Version),
			"at":      instant(event.At),
		}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func productDefinitionFields(product domain.ProductID, partner domain.PartnerID, agreement domain.AgreementID, code string, version int,
	validFrom, validUntil kernel.Instant, assetTypes, regions []string, currency string,
	minAmount, maxAmount int64, minTerm, maxTerm int, at kernel.Instant,
) domainkit.Fields {
	return domainkit.Fields{
		"product":          string(product),
		"partner":          string(partner),
		"agreement":        string(agreement),
		"code":             code,
		"version":          strconv.Itoa(version),
		"valid_from":       instant(validFrom),
		"valid_until":      instant(validUntil),
		"asset_types":      strings.Join(assetTypes, ","),
		"regions":          strings.Join(regions, ","),
		"currency":         currency,
		"min_amount_minor": strconv.FormatInt(minAmount, 10),
		"max_amount_minor": strconv.FormatInt(maxAmount, 10),
		"min_term_days":    strconv.Itoa(minTerm),
		"max_term_days":    strconv.Itoa(maxTerm),
		"at":               instant(at),
	}
}
