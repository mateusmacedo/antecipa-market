// Package ports is the port block of the receivables-gateway bounded
// context: the declared boundary to state, outbox, inbox, raw store and
// partners. It realizes nothing but the capture a submission carries and the
// absence of partner adapters the runtime starts with.
// Its unit and its bounded context are stated in dmpf-units.json (ADR-012).
package ports
