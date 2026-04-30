package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
)

type Handler struct {
	nc        *nats.Conn
	jwtSecret string
}

func NewHandler(nc *nats.Conn, jwtSecret string) *Handler {
	return &Handler{nc: nc, jwtSecret: jwtSecret}
}

// Token handles POST /api/v1/auth/token — issues a signed JWT for dashboard access.
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil || creds.Username == "" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	token := h.signToken(creds.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// MESWebhook handles POST /api/v1/webhooks/mes — ingests Level 4 MES state changes
// from legacy factory software and forwards them to the NATS event stream.
func (h *Handler) MESWebhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		StationID string `json:"station_id"`
		VIN       string `json:"vin"`
		Firmware  string `json:"firmware"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.StationID == "" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	data, _ := json.Marshal(payload)
	if err := h.nc.Publish("aegis.mes.state", data); err != nil {
		log.Printf("rest: nats publish mes state: %v", err)
		http.Error(w, `{"error":"upstream error"}`, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"accepted":true}`))
}

// signToken produces an HS256-signed JWT.
func (h *Handler) signToken(subject string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": subject,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(8 * time.Hour).Unix(),
	})
	payload := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig)
}
