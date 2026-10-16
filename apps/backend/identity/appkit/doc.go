// Package appkit composes the identity-organization application with its
// concrete realizations and observes the effects a use case leaves behind
// (the edge-to-edge kit: one service with its real internal wiring and no peer
// services). App block; its tests carry the integration build tag.
//
// Compose the service here the way the composition root does, minus the
// instrumentation: a harness observes effects, not telemetry. For a producing
// context the edge observed is the outbox; for a consuming one it is the
// inbox and the broker gesture.
package appkit
