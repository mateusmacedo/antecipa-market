package application

// CommandConsumer names the inbox entries of this context's commands, each
// identified by tenant and idempotency key:
// the use case registers under it, and serve-api purges what expired.
const CommandConsumer = "identity.commands"
