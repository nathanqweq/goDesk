package topdesk

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HealthCheck testa a API do TopDesk com uma consulta leve
// (GET /tas/api/incidents?pageSize=1) e devolve o status HTTP e a latência
// observada. Usado pela rotina periódica do godesk serve.
func (c Client) HealthCheck() (statusCode int, elapsed time.Duration, err error) {
	url := strings.TrimRight(c.BaseURL, "/") + "/tas/api/incidents?pageSize=1"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader())

	start := time.Now()
	resp, err := c.HTTP.Do(req)
	elapsed = time.Since(start)
	if err != nil {
		return 0, elapsed, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, elapsed, fmt.Errorf("TopDesk HealthCheck HTTP %d", resp.StatusCode)
	}

	return resp.StatusCode, elapsed, nil
}
