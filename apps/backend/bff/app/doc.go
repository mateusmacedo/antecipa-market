// Package app is the public REST edge of the workspace (kernel ADR-024): it
// translates each route into a gRPC call to a bounded context and owns no domain.
package app
