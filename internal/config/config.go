package config

import (
	"bufio"
	"errors"
	"os"
	"strings"
	"time"
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
const serviceConfigEnvPath = "/etc/zabbix/godesk/godesk-service.env"

func FromArgs(argv []string) (RuntimeConfig, error) {
	// TICKET_NAME RAWDATA — DOMAIN/USER/PASS/ZABBIX_URL/ZABBIX_KEY não são
	// mais parâmetros: vêm só de godesk-service.env (ServiceConfigFromEnv,
	// carregado uma vez aqui).
	if len(argv) < 3 {
		return RuntimeConfig{}, errors.New("parâmetros insuficientes: esperado 2 args (TICKET_NAME RAWDATA)")
	}

	svc := ServiceConfigFromEnv()
	return FromValues(svc, argv[1], argv[2]), nil
}

// FromValues monta o RuntimeConfig a partir do ServiceConfig já carregado
// (uma vez, no início do processo — ver ServiceConfigFromEnv) mais os dois
// valores que de fato mudam por alerta: ticketName e rawData.
func FromValues(svc ServiceConfig, ticketName, rawData string) RuntimeConfig {
	smtpFromFile := LoadEnvFile(smtpConfigEnvPath)

	cfg := RuntimeConfig{
		Domain:     svc.TopdeskDomain,
		User:       svc.TopdeskUser,
		Pass:       svc.TopdeskPass,
		TicketName: ticketName,
		RawData:    rawData,
		ZabbixURL:  svc.ZabbixURL,
		ZabbixKey:  svc.ZabbixKey,

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

// ServiceConfig é a configuração central do goDesk — carregada uma única
// vez no início do processo (ServiceConfigFromEnv) e reaproveitada dali em
// diante, tanto pelo godesk serve quanto pelo modo one-shot (FromArgs).
type ServiceConfig struct {
	ListenAddr string
	Token      string
	LogFile    string
	ConfigFile string

	// TopDesk padrão do serviço — não vem mais de alerta nenhum, é sempre
	// daqui (usado tanto pra criar/atualizar chamados quanto pelo
	// healthcheck em background).
	TopdeskDomain string
	TopdeskUser   string
	TopdeskPass   string

	// Zabbix padrão do serviço (URL da API + token do event.acknowledge).
	ZabbixURL string
	ZabbixKey string

	// HealthCheckInterval <= 0 desliga o healthcheck periódico do TopDesk.
	HealthCheckInterval time.Duration
}

// ServiceConfigFromEnv monta o ServiceConfig a partir do ambiente do
// processo, com godesk-service.env como fallback pra cada chave (systemd
// injeta via EnvironmentFile= quando roda como serviço; o modo one-shot,
// que não passa pelo systemd, lê o arquivo direto). Chame uma vez só, no
// início do processo — não é pra recarregar a cada alerta.
func ServiceConfigFromEnv() ServiceConfig {
	fromFile := LoadEnvFile(serviceConfigEnvPath)
	get := func(key, def string) string { return getenv(key, pickDefault(fromFile[key], def)) }

	return ServiceConfig{
		ListenAddr: get("GODESK_LISTEN_ADDR", "127.0.0.1:8787"),
		Token:      get("GODESK_SERVICE_TOKEN", ""),
		LogFile:    get("TOPDESK_LOG_FILE", "/var/log/godesk/godesk-service.log"),
		ConfigFile: get("TOPDESK_CONFIG", "/etc/zabbix/godesk/godesk-config.yaml"),

		TopdeskDomain: get("GODESK_TOPDESK_DOMAIN", ""),
		TopdeskUser:   get("GODESK_TOPDESK_USER", ""),
		TopdeskPass:   get("GODESK_TOPDESK_PASS", ""),

		ZabbixURL: get("GODESK_ZABBIX_URL", ""),
		ZabbixKey: get("GODESK_ZABBIX_KEY", ""),

		HealthCheckInterval: parseDurationDefault(get("GODESK_HEALTHCHECK_INTERVAL", ""), 0),
	}
}

// parseDurationDefault interpreta strings tipo "5m"/"30s"; vazio ou
// inválido devolve def sem erro (nunca derruba o serviço por causa disso).
func parseDurationDefault(s string, def time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
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
