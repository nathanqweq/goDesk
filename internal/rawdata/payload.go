package rawdata

type Payload struct {
	Status          string `json:"status"`
	Host            string `json:"host"`
	Trigger         string `json:"trigger"`
	ValueItem       string `json:"value_item"`
	Severity        string `json:"severity"`
	Date            string `json:"date"`
	Hour            string `json:"hour"`
	TriggerID       string `json:"trigger_id"`
	EventID         string `json:"event_id"`
	Contract        string `json:"contract"`
	OperGroup       string `json:"oper_group"`
	MainCaller      string `json:"main_caller"`
	SecundaryCaller string `json:"secundary_caller"`
	Cliente         string `json:"cliente"`
	EventValue      string `json:"event_value"` // {EVENT.VALUE} 1/0
}
