package zabbix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	Timeout time.Duration

	// Se true, ignora validação TLS APENAS para chamadas do Zabbix
	InsecureTLS bool
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcErr         `json:"error"`
	ID      any             `json:"id"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

// event.acknowledge action (bitmask)
const (
	ackActionAcknowledge = 2
	ackActionMessage     = 4
)

// ErrNotConfigured indica que GODESK_ZABBIX_URL/KEY não estão configurados.
var ErrNotConfigured = errors.New("zabbix não configurado (url/token vazios)")

// Acknowledge reconhece o evento e adiciona message (action=6) — encerra
// escalonamentos de Actions com condição "não reconhecido".
func (c Client) Acknowledge(eventID string, message string) error {
	return c.acknowledge(eventID, ackActionAcknowledge|ackActionMessage, message)
}

// AddMessage só comenta no evento (action=4), SEM reconhecer: o evento
// continua "não reconhecido" e um escalonamento da Action no Zabbix pode
// reenviar o alerta mais tarde (usado pela dependência de hosts, pra
// reavaliar quando o pai normalizar).
func (c Client) AddMessage(eventID string, message string) error {
	return c.acknowledge(eventID, ackActionMessage, message)
}

func (c Client) acknowledge(eventID string, action int, message string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !c.configured() {
		log.Printf("[zabbix] ACK SKIP base_set=%v token_set=%v eventid=%q\n",
			strings.TrimSpace(c.BaseURL) != "", strings.TrimSpace(c.Token) != "", eventID)
		return nil
	}

	_, err := c.call("event.acknowledge", map[string]any{
		"eventids": []string{eventID},
		"action":   action,
		"message":  message,
	})
	if err != nil {
		return fmt.Errorf("Zabbix ack eventid=%s: %w", eventID, err)
	}

	log.Printf("[zabbix] ACK OK eventid=%q action=%d\n", eventID, action)
	return nil
}

// EventAction faz várias ações num evento de uma vez (bloco "ação no
// Zabbix" dos fluxos): comentário (message), reconhecer (ack), mudar
// severidade (0-5; < 0 = não muda) e suprimir até suppressUntil (unix;
// 0 = não suprime — Zabbix 6.2+). Nada pra fazer = nada é chamado.
func (c Client) EventAction(eventID, message string, ack bool, severity int, suppressUntil int64) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !c.configured() {
		return nil
	}
	action := 0
	params := map[string]any{"eventids": []string{eventID}}
	if strings.TrimSpace(message) != "" {
		action |= ackActionMessage
		params["message"] = message
	}
	if ack {
		action |= ackActionAcknowledge
	}
	if severity >= 0 && severity <= 5 {
		action |= 8
		params["severity"] = severity
	}
	if suppressUntil > 0 {
		action |= 32
		params["suppress_until"] = suppressUntil
	}
	if action == 0 {
		return nil
	}
	params["action"] = action
	if _, err := c.call("event.acknowledge", params); err != nil {
		return fmt.Errorf("Zabbix event.acknowledge eventid=%s action=%d: %w", eventID, action, err)
	}
	log.Printf("[zabbix] AÇÃO OK eventid=%q action=%d\n", eventID, action)
	return nil
}

// Host é o mínimo de host.get que o goDesk precisa.
type Host struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
}

// HostsByName resolve nomes de host pra hostid: primeiro pelo nome técnico
// ("host"), e o que não achar tenta pelo nome visível ("name"). Nomes sem
// correspondência simplesmente não aparecem no resultado.
func (c Client) HostsByName(names []string) (map[string]Host, error) {
	out := map[string]Host{}
	if len(names) == 0 {
		return out, nil
	}

	pending := names
	for _, field := range []string{"host", "name"} {
		if len(pending) == 0 {
			break
		}

		raw, err := c.call("host.get", map[string]any{
			"output": []string{"hostid", "host", "name"},
			"filter": map[string]any{field: pending},
		})
		if err != nil {
			return nil, err
		}

		var hosts []Host
		if err := json.Unmarshal(raw, &hosts); err != nil {
			return nil, fmt.Errorf("host.get: %w", err)
		}

		var next []string
		for _, n := range pending {
			found := false
			for _, h := range hosts {
				v := h.Host
				if field == "name" {
					v = h.Name
				}
				if strings.EqualFold(v, n) {
					out[strings.ToLower(n)] = h
					found = true
					break
				}
			}
			if !found {
				next = append(next, n)
			}
		}
		pending = next
	}

	return out, nil
}

// Tag é uma tag do Zabbix (host, trigger ou evento).
type Tag struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

// HostInfo é o contexto do host usado pelos fluxos (campos host.name,
// host.tag.X e host.group).
type HostInfo struct {
	HostID string
	Host   string
	Name   string
	Tags   []Tag
	Groups []string
}

// HostInfo busca nome visível, tags e grupos do host pelo hostid (ou, sem
// hostid, pelo nome técnico). Uma chamada host.get. Host inexistente =
// HostInfo vazio, sem erro.
func (c Client) HostInfo(hostID, host string) (HostInfo, error) {
	params := map[string]any{
		"output":           []string{"hostid", "host", "name"},
		"selectTags":       []string{"tag", "value"},
		"selectHostGroups": []string{"name"},
	}
	switch {
	case strings.TrimSpace(hostID) != "":
		params["hostids"] = []string{hostID}
	case strings.TrimSpace(host) != "":
		params["filter"] = map[string]any{"host": []string{host}}
	default:
		return HostInfo{}, nil
	}

	// tags herdadas dos templates também contam como tag do host; Zabbix
	// que não conhece selectInheritedTags recusa o parâmetro → tenta sem
	params["selectInheritedTags"] = []string{"tag", "value"}
	raw, err := c.call("host.get", params)
	if err != nil {
		delete(params, "selectInheritedTags")
		raw, err = c.call("host.get", params)
	}
	if err != nil {
		return HostInfo{}, err
	}

	var hosts []struct {
		HostID        string `json:"hostid"`
		Host          string `json:"host"`
		Name          string `json:"name"`
		Tags          []Tag  `json:"tags"`
		InheritedTags []Tag  `json:"inheritedTags"`
		HostGroups    []struct {
			Name string `json:"name"`
		} `json:"hostgroups"`
	}
	if err := json.Unmarshal(raw, &hosts); err != nil {
		return HostInfo{}, fmt.Errorf("host.get: %w", err)
	}
	if len(hosts) == 0 {
		return HostInfo{}, nil
	}

	h := hosts[0]
	info := HostInfo{HostID: h.HostID, Host: h.Host, Name: h.Name, Tags: mergeTags(h.Tags, h.InheritedTags)}
	for _, g := range h.HostGroups {
		info.Groups = append(info.Groups, g.Name)
	}
	return info, nil
}

// mergeTags junta as tags do host com as herdadas, sem repetir tag=valor
// (as do host primeiro).
func mergeTags(own, inherited []Tag) []Tag {
	out := append([]Tag(nil), own...)
	seen := map[string]bool{}
	for _, t := range own {
		seen[strings.ToLower(t.Tag)+"\x00"+t.Value] = true
	}
	for _, t := range inherited {
		k := strings.ToLower(t.Tag) + "\x00" + t.Value
		if !seen[k] {
			seen[k] = true
			out = append(out, t)
		}
	}
	return out
}

// EventInfo é o contexto do evento usado pelos fluxos (event.tag.X,
// event.age, event.acknowledged, event.acked_by_user).
type EventInfo struct {
	Found        bool
	Tags         []Tag
	Clock        time.Time // início do problema
	Acknowledged bool      // reconhecido por qualquer um (inclusive o goDesk)
	AckedByUser  bool      // reconhecido por alguém além do goDesk
}

// mensagens que o próprio goDesk grava ao dar ack — identificam o usuário
// do token pra separar o ack do goDesk do ack do atendimento
var godeskAckPrefixes = []string{"chamado ", "normalização recebida", "normalizacao recebida", "godesk"}

// EventInfo busca tags, início, status de ack e quem reconheceu o evento.
// Uma chamada event.get. Evento inexistente = EventInfo{Found: false}.
func (c Client) EventInfo(eventID string) (EventInfo, error) {
	if strings.TrimSpace(eventID) == "" {
		return EventInfo{}, nil
	}
	raw, err := c.call("event.get", map[string]any{
		"output":              []string{"eventid", "clock", "acknowledged"},
		"eventids":            []string{eventID},
		"selectTags":          []string{"tag", "value"},
		"select_acknowledges": []string{"userid", "action", "message"},
	})
	if err != nil {
		return EventInfo{}, err
	}

	var events []struct {
		Clock        string `json:"clock"`
		Acknowledged string `json:"acknowledged"`
		Tags         []Tag  `json:"tags"`
		Acks         []struct {
			UserID  string `json:"userid"`
			Action  string `json:"action"`
			Message string `json:"message"`
		} `json:"acknowledges"`
	}
	if err := json.Unmarshal(raw, &events); err != nil {
		return EventInfo{}, fmt.Errorf("event.get: %w", err)
	}
	if len(events) == 0 {
		return EventInfo{}, nil
	}

	e := events[0]
	info := EventInfo{Found: true, Tags: e.Tags, Acknowledged: e.Acknowledged == "1"}
	if sec, err := strconv.ParseInt(e.Clock, 10, 64); err == nil && sec > 0 {
		info.Clock = time.Unix(sec, 0)
	}

	// usuários que gravaram mensagens do goDesk = o próprio goDesk
	godesk := map[string]bool{}
	for _, a := range e.Acks {
		msg := strings.ToLower(strings.TrimSpace(a.Message))
		for _, p := range godeskAckPrefixes {
			if strings.HasPrefix(msg, p) {
				godesk[a.UserID] = true
				break
			}
		}
	}
	for _, a := range e.Acks {
		action, _ := strconv.Atoi(a.Action)
		if action&2 != 0 && !godesk[a.UserID] {
			info.AckedByUser = info.Acknowledged
			break
		}
	}
	return info, nil
}

// Problem é um problema ativo (problem.get).
type Problem struct {
	EventID  string `json:"eventid"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Clock    string `json:"clock"`
}

// ActiveProblems devolve os problemas ativos (não resolvidos) do host,
// mais recentes primeiro.
func (c Client) ActiveProblems(hostID string) ([]Problem, error) {
	raw, err := c.call("problem.get", map[string]any{
		"output":    []string{"eventid", "name", "severity", "clock"},
		"hostids":   []string{hostID},
		"source":    0,
		"object":    0,
		"sortfield": []string{"eventid"},
		"sortorder": "DESC",
	})
	if err != nil {
		return nil, err
	}

	var problems []Problem
	if err := json.Unmarshal(raw, &problems); err != nil {
		return nil, fmt.Errorf("problem.get: %w", err)
	}
	return problems, nil
}

// ActiveProblemsByHost devolve os problemas ativos de vários hosts de uma
// vez (usado pela dependência v2, que precisa do estado da localidade
// inteira): problem.get não diz de qual host é cada problema, então um
// trigger.get pelos objectid faz o mapa trigger → host. Duas chamadas no
// total, independente da quantidade de hosts.
func (c Client) ActiveProblemsByHost(hostIDs []string) (map[string][]Problem, error) {
	out := map[string][]Problem{}
	if len(hostIDs) == 0 {
		return out, nil
	}

	raw, err := c.call("problem.get", map[string]any{
		"output":    []string{"eventid", "name", "severity", "clock", "objectid"},
		"hostids":   hostIDs,
		"source":    0,
		"object":    0,
		"sortfield": []string{"eventid"},
		"sortorder": "DESC",
	})
	if err != nil {
		return nil, err
	}

	var problems []struct {
		Problem
		ObjectID string `json:"objectid"`
	}
	if err := json.Unmarshal(raw, &problems); err != nil {
		return nil, fmt.Errorf("problem.get: %w", err)
	}
	if len(problems) == 0 {
		return out, nil
	}

	triggerIDs := make([]string, 0, len(problems))
	seen := map[string]bool{}
	for _, p := range problems {
		if !seen[p.ObjectID] {
			seen[p.ObjectID] = true
			triggerIDs = append(triggerIDs, p.ObjectID)
		}
	}

	raw, err = c.call("trigger.get", map[string]any{
		"output":      []string{"triggerid"},
		"triggerids":  triggerIDs,
		"selectHosts": []string{"hostid"},
	})
	if err != nil {
		return nil, err
	}

	var triggers []struct {
		TriggerID string `json:"triggerid"`
		Hosts     []struct {
			HostID string `json:"hostid"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(raw, &triggers); err != nil {
		return nil, fmt.Errorf("trigger.get: %w", err)
	}

	hostsOf := map[string][]string{}
	for _, t := range triggers {
		for _, h := range t.Hosts {
			hostsOf[t.TriggerID] = append(hostsOf[t.TriggerID], h.HostID)
		}
	}

	wanted := map[string]bool{}
	for _, id := range hostIDs {
		wanted[id] = true
	}
	for _, p := range problems {
		for _, hid := range hostsOf[p.ObjectID] {
			if wanted[hid] {
				out[hid] = append(out[hid], p.Problem)
			}
		}
	}
	return out, nil
}

func (c Client) configured() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Token) != ""
}

// call faz uma chamada JSON-RPC (token Bearer no header) e devolve o
// "result" cru, tratando erro HTTP e erro JSON-RPC (que vem com HTTP 200).
func (c Client) call(method string, params any) (json.RawMessage, error) {
	if !c.configured() {
		return nil, ErrNotConfigured
	}

	to := c.Timeout
	if to <= 0 {
		to = 10 * time.Second
	}

	url := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/") + "/api_jsonrpc.php"

	b, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))

	// client só pro Zabbix (não afeta TopDesk)
	baseClient := c.HTTP
	if baseClient == nil {
		baseClient = &http.Client{}
	}

	zclient := &http.Client{
		Timeout: baseClient.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.InsecureTLS},
		},
	}

	log.Printf("[zabbix] %s SEND url=%q insecureTLS=%v timeout=%s\n", method, url, c.InsecureTLS, to)

	resp, err := zclient.Do(req)
	if err != nil {
		log.Printf("[zabbix] %s ERROR http_do url=%q err=%v\n", method, url, err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	// limita body no log pra não explodir
	bodyLog := string(bodyBytes)
	if len(bodyLog) > 800 {
		bodyLog = bodyLog[:800] + "...(truncated)"
	}

	log.Printf("[zabbix] %s HTTP status=%d resp=%s\n", method, resp.StatusCode, bodyLog)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Zabbix %s HTTP %d: %s", method, resp.StatusCode, bodyLog)
	}

	// valida erro JSON-RPC mesmo com HTTP 200
	var rr rpcResp
	if err := json.Unmarshal(bodyBytes, &rr); err != nil {
		return nil, fmt.Errorf("Zabbix %s: resposta não é JSON-RPC válida: %v (body=%s)", method, err, bodyLog)
	}
	if rr.Error != nil {
		return nil, fmt.Errorf("Zabbix %s RPC error code=%d message=%q data=%q",
			method, rr.Error.Code, rr.Error.Message, rr.Error.Data)
	}

	return rr.Result, nil
}
