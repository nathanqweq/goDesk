package topdesk

import "testing"

func TestParseIncidentListArray(t *testing.T) {
	body := []byte(`[{"number":"INC-123","processingStatus":{"name":"Registrado"}}]`)

	incidents, err := parseIncidentList(body)
	if err != nil {
		t.Fatalf("parseIncidentList returned error: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Number != "INC-123" {
		t.Fatalf("expected INC-123, got %q", incidents[0].Number)
	}
}

func TestParseIncidentListObject(t *testing.T) {
	body := []byte(`{"number":"INC-456","processingStatus":{"name":"Em andamento"}}`)

	incidents, err := parseIncidentList(body)
	if err != nil {
		t.Fatalf("parseIncidentList returned error: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].ProcessingStatus.Name != "Em andamento" {
		t.Fatalf("expected status Em andamento, got %q", incidents[0].ProcessingStatus.Name)
	}
}

func TestParseIncidentListWrappedResults(t *testing.T) {
	body := []byte(`{"results":[{"number":"INC-789","processingStatus":{"name":"Registrado"}}]}`)

	incidents, err := parseIncidentList(body)
	if err != nil {
		t.Fatalf("parseIncidentList returned error: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Number != "INC-789" {
		t.Fatalf("expected INC-789, got %q", incidents[0].Number)
	}
}
