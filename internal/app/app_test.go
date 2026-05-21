package app

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildTicketNameKeepsSubjectWhenWithin85Chars(t *testing.T) {
	subject := "[TELTEC]: Interface down - 12345"

	got := buildTicketName(subject)

	if got != subject {
		t.Fatalf("unexpected ticket name: %q", got)
	}
}

func TestBuildTicketNameTrimsOnlyEventNameWhenSubjectPasses85Chars(t *testing.T) {
	eventName := strings.Repeat("A", 100)
	hostID := "12345"
	subject := fmt.Sprintf("[TELTEC]: %s - %s", eventName, hostID)

	got := buildTicketName(subject)

	if utf8.RuneCountInString(got) != 85 {
		t.Fatalf("expected ticket name with 85 chars, got %d: %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasPrefix(got, "[TELTEC]: ") {
		t.Fatalf("expected ticket name to keep client prefix, got %q", got)
	}
	if !strings.HasSuffix(got, " - "+hostID) {
		t.Fatalf("expected ticket name to keep host ID %q, got %q", hostID, got)
	}
}

func TestBuildTicketNameUsesExpectedPattern(t *testing.T) {
	got := buildTicketName("[TELTEC]: Interface Vl150(## VLAN UCS MGMT ##): High inbound bandwidth usage - 10293847")

	want := "[TELTEC]: Interface Vl150(## VLAN UCS MGMT ##): High inbound bandwidth usa - 10293847"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if utf8.RuneCountInString(got) != 85 {
		t.Fatalf("expected ticket name with 85 chars, got %d", utf8.RuneCountInString(got))
	}
}

func TestBuildTicketNameFallbackTruncatesUnexpectedPattern(t *testing.T) {
	got := buildTicketName(strings.Repeat("B", 90))

	if utf8.RuneCountInString(got) != 85 {
		t.Fatalf("expected fallback ticket name with 85 chars, got %d", utf8.RuneCountInString(got))
	}
}
