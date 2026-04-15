package execution

import (
    "bytes"
    "encoding/json"
    "net/http"
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
        url:    riskEngineURL + "/validate",
    }
}

func (g *RiskGate) Validate(order models.Order) error {
    body, err := json.Marshal(order)
    if err != nil {
        return err
    }
    resp, err := g.client.Post(g.url, "application/json", bytes.NewReader(body))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return ErrRiskRejected
    }
    return nil
}
