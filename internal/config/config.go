package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

type RuntimeConfig struct {
	Domain     string
	User       string
	Pass       string
	TicketName string
	RawData    string
	ZabbixURL  string
	ZabbixKey  string

	LogFile    string
	ConfigFile string
	TimeoutSec int

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

const smtpConfigEnvPath = "/etc/zabbix/godesk/godesk-smtp-config.env"

func FromArgs(argv []string) (RuntimeConfig, error) {
	// DOMAIN USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY
	if len(argv) < 8 {
		return RuntimeConfig{}, errors.New("parâmetros insuficientes: esperado 7 args (DOMAIN USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY)")
	}

	smtpFromFile := loadEnvFile(smtpConfigEnvPath)

	cfg := RuntimeConfig{
		Domain:     argv[1],
		User:       argv[2],
		Pass:       argv[3],
		TicketName: argv[4],
		RawData:    argv[5],
		ZabbixURL:  argv[6],
		ZabbixKey:  argv[7],

		LogFile:    getenv("TOPDESK_LOG_FILE", "/tmp/goDesk-integration.log"),
		ConfigFile: getenv("TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml"),
		TimeoutSec: atoiDefault(getenv("TOPDESK_TIMEOUT_SEC", "15"), 15),
		SMTPHost:   getenv("TOPDESK_SMTP_HOST", smtpFromFile["TOPDESK_SMTP_HOST"]),
		SMTPPort:   getenv("TOPDESK_SMTP_PORT", pickDefault(smtpFromFile["TOPDESK_SMTP_PORT"], "25")),
		SMTPUser:   getenv("TOPDESK_SMTP_USER", smtpFromFile["TOPDESK_SMTP_USER"]),
		SMTPPass:   getenv("TOPDESK_SMTP_PASS", smtpFromFile["TOPDESK_SMTP_PASS"]),
		SMTPFrom:   getenv("TOPDESK_SMTP_FROM", smtpFromFile["TOPDESK_SMTP_FROM"]),
	}

	// sane
	cfg.Domain = strings.TrimRight(cfg.Domain, "/")
	cfg.ZabbixURL = strings.TrimRight(cfg.ZabbixURL, "/")

	return cfg, nil
}

func getenv(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

func pickDefault(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return fallback
}

func loadEnvFile(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		// aceita valores terminando com comentário inline: KEY="x" # comment
		if idx := strings.Index(val, " #"); idx >= 0 {
			val = strings.TrimSpace(val[:idx])
			val = strings.Trim(val, `"'`)
		}

		out[key] = val
	}
	return out
}
