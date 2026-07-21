package mailer

import (
	"errors"
	"net/smtp"
)

// loginAuth implementa o mecanismo "AUTH LOGIN" — net/smtp só traz PLAIN e
// CRAM-MD5 prontos, mas o relay SMTP do Azure Communication Services
// (smtp.azurecomm.net, usado em produção aqui) só aceita LOGIN e responde
// "504 5.7.4 Unrecognized authentication type" pra AUTH PLAIN.
type loginAuth struct {
	username string
	password string
}

// LoginAuth devolve um smtp.Auth que fala o mecanismo LOGIN.
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username: username, password: password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	switch string(fromServer) {
	case "Username:":
		return []byte(a.username), nil
	case "Password:":
		return []byte(a.password), nil
	default:
		return nil, errors.New("mailer: prompt inesperado do servidor no AUTH LOGIN: " + string(fromServer))
	}
}
