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

func TestBuildTicketNameTrimsOnlyEventNameWhenSubjectPasses80Chars(t *testing.T) {
	eventName := strings.Repeat("A", 100)
	hostID := "12345"
	subject := fmt.Sprintf("[TELTEC]: %s - %s", eventName, hostID)

	got := buildTicketName(subject)

	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected ticket name with 80 chars, got %d: %q", utf8.RuneCountInString(got), got)
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

	want := "[TELTEC]: Interface Vl150(## VLAN UCS MGMT ##): High inbound bandwidt - 10293847"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected ticket name with 80 chars, got %d", utf8.RuneCountInString(got))
	}
}

func TestBuildTicketNameFallbackTruncatesUnexpectedPattern(t *testing.T) {
	got := buildTicketName(strings.Repeat("B", 90))

	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected fallback ticket name with 80 chars, got %d", utf8.RuneCountInString(got))
	}
}

func TestBuildTicketNameFitsTopDeskBriefDescriptionLimit(t *testing.T) {
	got := buildTicketName("[CRESOL]: Este alerta é apenas um teste para verificar a correção nos limites - 16149")

	want := "[CRESOL]: Este alerta é apenas um teste para verificar a correção nos li - 16149"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if utf8.RuneCountInString(got) != 80 {
		t.Fatalf("expected ticket name with 80 chars, got %d", utf8.RuneCountInString(got))
	}
}

func TestBuildTicketNameNeverPasses80IncludingHostID(t *testing.T) {
	hostID := "12345"
	got := buildTicketName("[CRESOL]: " + strings.Repeat("A", 200) + " - " + hostID)

	if utf8.RuneCountInString(got) > 80 {
		t.Fatalf("expected ticket name with at most 80 chars, got %d: %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasSuffix(got, " - "+hostID) {
		t.Fatalf("expected ticket name to keep host ID at end, got %q", got)
	}
}
