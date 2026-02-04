package topdesk

type Incident struct {
	Number           string `json:"number"`
	ProcessingStatus struct {
		Name string `json:"name"`
	} `json:"processingStatus"`
}
