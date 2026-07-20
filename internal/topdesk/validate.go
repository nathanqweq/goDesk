package topdesk

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// IDExists confere se um recurso existe no TopDesk via
// GET /tas/api/{resource}/id/{id}. resource é o nome da coleção REST (ex:
// "operators", "operatorgroups") — confirmado contra a API real do
// TopDesk para esses dois; não use para recursos não testados sem
// confirmar o caminho primeiro (nem todo recurso segue esse padrão — ex:
// SLA não respondeu em /tas/api/slas).
func (c Client) IDExists(resource, id string) (bool, error) {
	u := strings.TrimRight(c.BaseURL, "/") + "/tas/api/" + resource + "/id/" + url.PathEscape(id)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", c.authHeader())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("TopDesk %s/id/%s: HTTP %d", resource, id, resp.StatusCode)
	}
}

// OperatorExists confere GET /tas/api/operators/id/{id}.
func (c Client) OperatorExists(id string) (bool, error) {
	return c.IDExists("operators", id)
}

// OperatorGroupExists confere GET /tas/api/operatorgroups/id/{id}.
func (c Client) OperatorGroupExists(id string) (bool, error) {
	return c.IDExists("operatorgroups", id)
}
