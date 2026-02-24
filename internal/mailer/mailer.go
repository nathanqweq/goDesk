package mailer

import (
	"fmt"
	"net/smtp"
	"strings"
)

type Config struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func ParseRecipients(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})

	seen := map[string]bool{}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func SendHTML(cfg Config, to []string, cc []string, subject string, htmlBody string) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("smtp host vazio")
	}
	if strings.TrimSpace(cfg.Port) == "" {
		return fmt.Errorf("smtp port vazio")
	}
	if strings.TrimSpace(cfg.From) == "" {
		return fmt.Errorf("smtp from vazio")
	}
	if len(to) == 0 {
		return fmt.Errorf("destinatarios (to) vazios")
	}

	addr := strings.TrimSpace(cfg.Host) + ":" + strings.TrimSpace(cfg.Port)
	allRecipients := append([]string{}, to...)
	allRecipients = append(allRecipients, cc...)

	var auth smtp.Auth
	if strings.TrimSpace(cfg.User) != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, strings.TrimSpace(cfg.Host))
	}

	msg := buildMIME(cfg.From, to, cc, subject, htmlBody)
	if err := smtp.SendMail(addr, auth, cfg.From, allRecipients, []byte(msg)); err != nil {
		return fmt.Errorf("smtp sendmail: %w", err)
	}
	return nil
}

func buildMIME(from string, to []string, cc []string, subject string, htmlBody string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	if len(cc) > 0 {
		b.WriteString("Cc: " + strings.Join(cc, ", ") + "\r\n")
	}
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return b.String()
}
