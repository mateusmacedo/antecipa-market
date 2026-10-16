// Package distkit is the distributed-flow harness with redelivery for the
// identity-organization context: it re-executes the test binary as separate
// operating-system processes over a real broker, since no test double may stand
// in for the broker in a distributed test, supervised by the
// kernel testkit/tb/dist, and decides on the final effect. App block; tests
// build with the integration and distributed tags.
//
// The vector follows the role of the context. A consuming one injects a
// deliberate redelivery and decides on the effect whether the same message
// produced the business effect more than once. A producing one
// has no inbox to decide that on: what two drains over a single outbox can
// get wrong is publishing the same record twice, and what the publication can
// get wrong is a payload that no longer hashes to what the outbox stored.
package distkit
