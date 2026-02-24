package config

import (
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

func FromArgs(argv []string) (RuntimeConfig, error) {
	// DOMAIN USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY
	if len(argv) < 8 {
		return RuntimeConfig{}, errors.New("parâmetros insuficientes: esperado 7 args (DOMAIN USER PASS TICKET_NAME RAWDATA ZABBIX_URL ZABBIX_KEY)")
	}

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
		SMTPHost:   getenv("TOPDESK_SMTP_HOST", ""),
		SMTPPort:   getenv("TOPDESK_SMTP_PORT", "25"),
		SMTPUser:   getenv("TOPDESK_SMTP_USER", ""),
		SMTPPass:   getenv("TOPDESK_SMTP_PASS", ""),
		SMTPFrom:   getenv("TOPDESK_SMTP_FROM", ""),
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
