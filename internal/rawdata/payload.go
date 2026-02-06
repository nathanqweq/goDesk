package rawdata

// Payload recebido do Zabbix via RAWDATA JSON
// Tudo aqui pode vir do Zabbix e sobrescrever YAML
type Payload struct {
	// ===== INFO DO EVENTO =====
	Status    string `json:"status"`
	Host      string `json:"host"`
	Trigger   string `json:"trigger"`
	ValueItem string `json:"value_item"`
	Severity  string `json:"severity"`
	Date      string `json:"date"`
	Hour      string `json:"hour"`

	TriggerID string `json:"trigger_id"`
	EventID   string `json:"event_id"`
	EventValue string `json:"event_value"` // 1 problem / 0 recovery

	Cliente  string `json:"cliente"`

	// ===== TAGS TOPDESK =====
	Contract        string `json:"contract"`
	OperGroup       string `json:"oper_group"`
	Operator        string `json:"operator"`
	MainCaller      string `json:"main_caller"`
	SecundaryCaller string `json:"secundary_caller"`

	// ===== NOVOS CAMPOS DINÂMICOS =====
	Sla         string `json:"sla"`
	Category    string `json:"category"`
	SubCategory string `json:"sub_category"`
	CallType    string `json:"call_type"`

	// futuro (se quiser depois)
	EntryType        string `json:"entry_type"`
	ProcessingStatus string `json:"processing_status"`
}
