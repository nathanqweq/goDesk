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

	LogFile     string
	ConfigFile  string
	MetricsFile string
	TimeoutSec  int

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

	return FromValues(argv[1], argv[2], argv[3], argv[4], argv[5], argv[6], argv[7]), nil
}

// FromValues monta o RuntimeConfig a partir dos mesmos 7 valores que hoje
// chegam via argv (modo CLI/one-shot) ou via corpo da requisição HTTP
// (modo serviço). As demais opções (log, timeout, SMTP, etc.) continuam
// vindo do ambiente do processo em ambos os casos.
func FromValues(domain, user, pass, ticketName, rawData, zabbixURL, zabbixKey string) RuntimeConfig {
	smtpFromFile := LoadEnvFile(smtpConfigEnvPath)

	cfg := RuntimeConfig{
		Domain:     domain,
		User:       user,
		Pass:       pass,
		TicketName: ticketName,
		RawData:    rawData,
		ZabbixURL:  zabbixURL,
		ZabbixKey:  zabbixKey,

		LogFile:     getenv("TOPDESK_LOG_FILE", "/tmp/goDesk-integration.log"),
		ConfigFile:  getenv("TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml"),
		MetricsFile: MetricsFileFromEnv(),
		TimeoutSec:  atoiDefault(getenv("TOPDESK_TIMEOUT_SEC", "15"), 15),
		SMTPHost:    getenv("TOPDESK_SMTP_HOST", smtpFromFile["TOPDESK_SMTP_HOST"]),
		SMTPPort:    getenv("TOPDESK_SMTP_PORT", pickDefault(smtpFromFile["TOPDESK_SMTP_PORT"], "25")),
		SMTPUser:    getenv("TOPDESK_SMTP_USER", smtpFromFile["TOPDESK_SMTP_USER"]),
		SMTPPass:    getenv("TOPDESK_SMTP_PASS", smtpFromFile["TOPDESK_SMTP_PASS"]),
		SMTPFrom:    getenv("TOPDESK_SMTP_FROM", smtpFromFile["TOPDESK_SMTP_FROM"]),
	}

	// sane
	cfg.Domain = strings.TrimRight(cfg.Domain, "/")
	cfg.ZabbixURL = strings.TrimRight(cfg.ZabbixURL, "/")

	return cfg
}

// ServiceConfig contém as opções do modo serviço (`godesk serve`).
type ServiceConfig struct {
	ListenAddr string
	Token      string
	LogFile    string
	ConfigFile string
}

// ServiceConfigFromEnv lê as opções do modo serviço a partir do ambiente do
// processo. Em produção, o systemd injeta essas variáveis via
// EnvironmentFile= antes de iniciar o binário.
func ServiceConfigFromEnv() ServiceConfig {
	return ServiceConfig{
		ListenAddr: getenv("GODESK_LISTEN_ADDR", "127.0.0.1:8787"),
		Token:      getenv("GODESK_SERVICE_TOKEN", ""),
		LogFile:    getenv("TOPDESK_LOG_FILE", "/var/log/godesk/godesk-service.log"),
		ConfigFile: getenv("TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml"),
	}
}

// MetricsFileFromEnv devolve o caminho do arquivo de métricas persistentes
// (internal/metrics), usado tanto pelo modo one-shot/serve (que gravam)
// quanto por `godesk --monitoring` (que só lê) — mesma env var e default
// nos três casos, pra garantir que todos apontem pro mesmo arquivo.
func MetricsFileFromEnv() string {
	return getenv("GODESK_METRICS_FILE", "/var/lib/godesk/godesk-metrics.json")
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

// LoadEnvFile lê um arquivo estilo ".env" (KEY=value, comentários com #,
// prefixo opcional "export "). Usado tanto para o env do SMTP quanto pelo
// cmd/godesk-client, que precisa ler seu próprio arquivo de configuração
// já que quem o invoca (PHP via exec()) não repassa variáveis de ambiente.
func LoadEnvFile(path string) map[string]string {
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
