package main

import (
	"bytes"
	"strings"
	"testing"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func TestRunRefusesToStartWithoutARole(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{role: "", lookup: lookup()}, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "--role api|relay|consumer") {
		t.Fatalf("run() = %d, stderr %q; want usage listing the three roles", code, errOut.String())
	}
}

func TestRunRefusesARoleThisContextDoesNotHave(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{role: "dispatcher", lookup: lookup("PG_DSN", "postgres://x")}, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "api|relay|consumer") {
		t.Fatalf("run() = %d, stderr %q; want the refusal to name the roles it has", code, errOut.String())
	}
}

func TestRunRefusesTheConsumerWithoutThePartnerChannel(t *testing.T) {
	var errOut bytes.Buffer

	code := run(options{role: "consumer", lookup: lookup("PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_INSECURE", "true")}, &errOut)

	if code != exitUsage || !strings.Contains(errOut.String(), "KAFKA_PARTNER_TOPIC") {
		t.Fatalf("run() = %d, stderr %q; want the missing partner topic named", code, errOut.String())
	}
}
