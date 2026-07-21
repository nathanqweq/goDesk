package mailer

import (
	"net/smtp"
	"testing"
)

func TestLoginAuthStartReturnsLoginMechanism(t *testing.T) {
	auth := LoginAuth("user@example.com", "s3cr3t")

	mech, resp, err := auth.Start(&smtp.ServerInfo{Name: "smtp.azurecomm.net", TLS: true})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if mech != "LOGIN" {
		t.Fatalf("esperava mecanismo LOGIN, veio %q", mech)
	}
	if resp != nil {
		t.Fatalf("esperava resposta inicial vazia, veio %q", resp)
	}
}

func TestLoginAuthNextAnswersUsernameThenPassword(t *testing.T) {
	auth := LoginAuth("user@example.com", "s3cr3t")

	got, err := auth.Next([]byte("Username:"), true)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if string(got) != "user@example.com" {
		t.Fatalf("esperava username na resposta, veio %q", got)
	}

	got, err = auth.Next([]byte("Password:"), true)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if string(got) != "s3cr3t" {
		t.Fatalf("esperava password na resposta, veio %q", got)
	}
}

func TestLoginAuthNextStopsWhenServerDone(t *testing.T) {
	auth := LoginAuth("user@example.com", "s3cr3t")

	got, err := auth.Next(nil, false)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != nil {
		t.Fatalf("esperava resposta vazia quando more=false, veio %q", got)
	}
}

func TestLoginAuthNextErrorsOnUnexpectedPrompt(t *testing.T) {
	auth := LoginAuth("user@example.com", "s3cr3t")

	if _, err := auth.Next([]byte("Algo inesperado:"), true); err == nil {
		t.Fatal("esperava erro para prompt não reconhecido do servidor")
	}
}
