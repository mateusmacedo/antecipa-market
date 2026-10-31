package app

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrInvalidDSN refuses a PG_DSN the driver cannot parse. It names the
// variable, never the value, which may carry a password.
var ErrInvalidDSN = errors.New("gateway: " + envDSN + " is not a valid connection string")

// The API reads the series of a relational precondition under the guard of
// ADR-067, so its sessions bound the wait on a lock below the deadline of a
// command and the life of a transaction left open; a value the environment
// sets in PG_DSN wins.
var sessionTimeouts = []struct{ name, value string }{
	{"lock_timeout", "2s"},
	{"idle_in_transaction_session_timeout", "30s"},
}

// withSessionTimeouts adds each session timeout PG_DSN does not set, as a
// runtime parameter the driver sends at startup, appended in the form the DSN
// came in so the rest of it reaches pgconn as the environment wrote it.
func withSessionTimeouts(dsn string) (string, error) {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return "", ErrInvalidDSN
	}
	isURL := strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
	for _, timeout := range sessionTimeouts {
		if sets(cfg, timeout.name) {
			continue
		}
		switch {
		case !isURL:
			dsn += " " + timeout.name + "=" + timeout.value
		case strings.Contains(dsn, "?"):
			dsn += "&" + timeout.name + "=" + timeout.value
		default:
			dsn += "?" + timeout.name + "=" + timeout.value
		}
	}
	return dsn, nil
}

// sets tells a timeout the environment already gives, as a parameter of its
// own or through the -c of options, which PGOPTIONS also feeds: an appended
// startup parameter would apply after options and override it.
func sets(cfg *pgconn.Config, name string) bool {
	if _, ok := cfg.RuntimeParams[name]; ok {
		return true
	}
	return strings.Contains(cfg.RuntimeParams["options"], name+"=")
}
