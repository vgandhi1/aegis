package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"aegis/mes-service/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://aegis:aegis@127.0.0.1:5432/aegis_mes?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	if err := store.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("nats: %v", err)
	}
	defer nc.Drain()

	repo := store.New(pool)
	h := &handler{repo: repo, nc: nc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/sessions", h.createSession)
	mux.HandleFunc("GET /api/v1/work-orders", h.listWorkOrders)
	mux.HandleFunc("GET /api/v1/status", h.status)

	addr := ":8080"
	if p := os.Getenv("HTTP_PORT"); p != "" {
		addr = ":" + p
	}
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("MES listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	go mockSessions(ctx, repo, nc, addr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	cancel()
	shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
}

type handler struct {
	repo *store.Repository
	nc   *nats.Conn
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *handler) status(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	n, err := h.repo.CountWorkOrders(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"service":"mes","work_orders":` + strconv.FormatInt(n, 10) + `}`))
}

type sessionReq struct {
	StationID string `json:"station_id"`
	VIN       string `json:"vin"`
	Firmware  string `json:"firmware"`
}

func (h *handler) createSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req sessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	id, err := h.repo.InsertWorkOrder(ctx, req.StationID, req.VIN, req.Firmware)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"station_id": req.StationID,
		"vin":        req.VIN,
		"firmware":   req.Firmware,
	})
	if err := h.nc.Publish("aegis.mes.state", payload); err != nil {
		log.Printf("nats publish: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":` + strconv.FormatInt(id, 10) + `}`))
}

func (h *handler) listWorkOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.repo.ListRecent(ctx, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(rows)
}

func mockSessions(ctx context.Context, repo *store.Repository, nc *nats.Conn, httpAddr string) {
	t := time.NewTicker(45 * time.Second)
	defer t.Stop()
	stations := []string{"5", "6"}
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st := stations[i%len(stations)]
			i++
			vin := randomVIN(i)
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := repo.InsertWorkOrder(cctx, st, vin, "v2.1.4")
			cancel()
			if err != nil {
				log.Printf("mock session db: %v", err)
				continue
			}
			payload, _ := json.Marshal(map[string]string{
				"station_id": st,
				"vin":        vin,
				"firmware":   "v2.1.4",
			})
			if err := nc.Publish("aegis.mes.state", payload); err != nil {
				log.Printf("mock session nats: %v", err)
			}
			log.Printf("mock MES session station=%s vin=%s (http %s)", st, vin, httpAddr)
		}
	}
}

func randomVIN(seq int) string {
	const prefix = "1HGBH41JXMN"
	s := strconv.Itoa(100000 + seq%900000)
	for len(s) < 6 {
		s = "0" + s
	}
	return prefix + s
}
