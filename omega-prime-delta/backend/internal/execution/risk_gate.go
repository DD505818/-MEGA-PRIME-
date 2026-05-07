package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
)

type RiskGate struct {
	client *http.Client
	url    string
}

func NewRiskGate(riskEngineURL string) *RiskGate {
	return &RiskGate{
		client: &http.Client{Timeout: 500 * time.Millisecond},
		url:    strings.TrimRight(riskEngineURL, "/") + "/validate",
	}
}

func (g *RiskGate) Validate(order models.Order) error {
	requestPayload := struct {
		Order models.Order `json:"order"`
	}{Order: order}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return err
	}

	resp, err := g.client.Post(g.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRiskServiceUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrRiskRejected
	}
	return nil
}
