package app

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildTicketNameKeepsHostIDAtEndWithin80Chars(t *testing.T) {
	triggerName := strings.Repeat("A", 100)
	hostID := "12345"

	got := buildTicketName("TELTEC", triggerName, hostID)

	if utf8.RuneCountInString(got) > 80 {
		t.Fatalf("expected ticket name with at most 80 chars, got %d: %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasPrefix(got, "[TELTEC]: ") {
		t.Fatalf("expected ticket name to start with client name, got %q", got)
	}
	if !strings.HasSuffix(got, " - "+hostID) {
		t.Fatalf("expected ticket name to end with host ID %q, got %q", hostID, got)
	}
}

func TestBuildTicketNameTrimsTriggerBeforeHostID(t *testing.T) {
	got := buildTicketName("Cliente", "trigger com espaco final     ", "987")

	if got != "[Cliente]: trigger com espaco final - 987" {
		t.Fatalf("unexpected ticket name: %q", got)
	}
}

func TestBuildTicketNameWithoutHostIDStillCapsAt80Chars(t *testing.T) {
	got := buildTicketName("Cliente", strings.Repeat("B", 90), "")

	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected ticket name with 80 chars, got %d", utf8.RuneCountInString(got))
	}
}

func TestBuildTicketNameLongHostIDPreservesHostIDEnd(t *testing.T) {
	hostID := strings.Repeat("1", 79) + "9"

	got := buildTicketName("Cliente", "trigger", hostID)

	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected ticket name with 80 chars, got %d", utf8.RuneCountInString(got))
	}
	if got != hostID {
		t.Fatalf("expected long host ID to occupy the ticket name, got %q", got)
	}
}

func TestBuildTicketNameUsesExpectedPattern(t *testing.T) {
	got := buildTicketName(
		"TELTEC",
		"Interface Vl150(## VLAN UCS MGMT ##): High inbound bandwidth usage",
		"10293847",
	)

	want := "[TELTEC]: Interface Vl150(## VLAN UCS MGMT ##): High inbound bandwidt - 10293847"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected ticket name with 80 chars, got %d", utf8.RuneCountInString(got))
	}
}
