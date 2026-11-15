package app

import (
	"testing"
	"time"

	"godesk/internal/rawdata"
)

func TestOncePerDaySeenTodayFalseWhenNotRecorded(t *testing.T) {
	if oncePerDaySeenToday("trigger-never-seen", "host-1") {
		t.Fatal("esperava false para combinação nunca registrada")
	}
}

func TestOncePerDayRecordThenSeenToday(t *testing.T) {
	oncePerDayRecord("Cliente X", "trigger-record", "host-a", "host-1")

	if !oncePerDaySeenToday("trigger-record", "host-1") {
		t.Fatal("esperava true logo após oncePerDayRecord")
	}
	if oncePerDaySeenToday("trigger-record", "host-2") {
		t.Fatal("host_id diferente não deveria contar como visto")
	}
	if oncePerDaySeenToday("trigger-outro", "host-1") {
		t.Fatal("trigger diferente não deveria contar como visto")
	}
}

func TestOncePerDaySeenTodayIgnoresStaleEntry(t *testing.T) {
	key := oncePerDayKey("trigger-stale", "host-1")

	oncePerDayMu.Lock()
	oncePerDaySeen[key] = OncePerDayEntry{
		Alert:      "trigger-stale",
		HostID:     "host-1",
		Day:        "2000-01-01",
		RecordedAt: time.Now(),
	}
	oncePerDayMu.Unlock()

	if oncePerDaySeenToday("trigger-stale", "host-1") {
		t.Fatal("entrada de um dia anterior não deveria contar como vista hoje")
	}
}

func TestOncePerDayPurgeRemovesStaleEntries(t *testing.T) {
	oncePerDayRecord("Cliente Y", "trigger-fresh", "host-b", "host-2")

	staleKey := oncePerDayKey("trigger-old", "host-3")
	oncePerDayMu.Lock()
	oncePerDaySeen[staleKey] = OncePerDayEntry{
		Alert:      "trigger-old",
		HostID:     "host-3",
		Day:        "2000-01-01",
		RecordedAt: time.Now(),
	}
	oncePerDayMu.Unlock()

	OncePerDayPurge()

	oncePerDayMu.Lock()
	_, staleStillThere := oncePerDaySeen[staleKey]
	_, freshStillThere := oncePerDaySeen[oncePerDayKey("trigger-fresh", "host-2")]
	oncePerDayMu.Unlock()

	if staleStillThere {
		t.Fatal("OncePerDayPurge deveria remover entrada de dia anterior")
	}
	if !freshStillThere {
		t.Fatal("OncePerDayPurge não deveria remover entrada de hoje")
	}
}

func TestOncePerDayListOnlyToday(t *testing.T) {
	oncePerDayRecord("Cliente Z", "trigger-list", "host-c", "host-9")

	staleKey := oncePerDayKey("trigger-list-old", "host-8")
	oncePerDayMu.Lock()
	oncePerDaySeen[staleKey] = OncePerDayEntry{
		Alert:      "trigger-list-old",
		HostID:     "host-8",
		Day:        "2000-01-01",
		RecordedAt: time.Now(),
	}
	oncePerDayMu.Unlock()

	found := false
	for _, e := range OncePerDayList() {
		if e.Alert == "trigger-list-old" {
			t.Fatal("OncePerDayList não deveria incluir entrada de dia anterior")
		}
		if e.Alert == "trigger-list" && e.HostID == "host-9" {
			found = true
		}
	}
	if !found {
		t.Fatal("OncePerDayList deveria incluir a entrada registrada hoje")
	}
}

func TestOncePerDayHostFallsBackToHostName(t *testing.T) {
	a := oncePerDayHost(rawdata.Payload{Host: "SW-A"})
	b := oncePerDayHost(rawdata.Payload{Host: "SW-B"})
	if a == b {
		t.Fatalf("hosts diferentes sem host_id caíram na mesma chave: %q", a)
	}
	if got := oncePerDayHost(rawdata.Payload{Host: "SW-A", HostID: "10501"}); got != "10501" {
		t.Fatalf("com host_id deveria usar o id, veio %q", got)
	}
	oncePerDayRecord("c", "Unavailable by ICMP ping", "SW-A", a)
	if !oncePerDaySeenToday("Unavailable by ICMP ping", a) {
		t.Fatal("SW-A deveria constar como enviado hoje")
	}
	if oncePerDaySeenToday("Unavailable by ICMP ping", b) {
		t.Fatal("SW-B não pode ser bloqueado pelo e-mail do SW-A")
	}
}
