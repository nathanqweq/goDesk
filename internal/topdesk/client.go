package topdesk

import (
	"encoding/base64"
	"net/http"
)

type Client struct {
	BaseURL string
	User    string
	Pass    string
	HTTP    *http.Client
}

func (c Client) authHeader() string {
	token := base64.StdEncoding.EncodeToString([]byte(c.User + ":" + c.Pass))
	return "Basic " + token
}
