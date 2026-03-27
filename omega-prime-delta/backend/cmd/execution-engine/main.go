package main

import (
  "bytes"
  "encoding/json"
  "log"
  "net/http"
  "os"
)

func estimateSlippage(order map[string]interface{}) float64 {
  data,_ := json.Marshal(order)
  resp, err := http.Post(os.Getenv("IMPACT_MODEL_URL")+"/predict", "application/json", bytes.NewBuffer(data))
  if err != nil { return 0.001 }
  defer resp.Body.Close()
  var result map[string]float64
  _ = json.NewDecoder(resp.Body).Decode(&result)
  return result["slippage"]
}

func main(){ log.Println("execution engine ready") }
