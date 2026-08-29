package webhooks

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/engine"
	"github.com/Laaaaksh/escalight/internal/notify"
)

func newTestIngestor(t *testing.T) (*Ingestor, *db.Store, *db.Service) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	store := db.NewStore(conn)

	u, _ := store.CreateUser("a@example.com", "Alice", "hash", false)
	p, _ := store.CreatePolicy("Primary", "", 0)
	store.ReplaceSteps(p.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{{TargetType: "user", TargetID: u.ID}}},
	})
	svc, _ := store.CreateService("API", p.ID)

	dispatcher := &notify.Dispatcher{
		Store: store, BaseURL: "http://localhost:8080",
		Email: &notify.EmailSender{}, Slack: &notify.SlackSender{}, Discord: &notify.DiscordSender{}, Push: &notify.WebPushSender{},
	}
	e := engine.New(store, dispatcher, nil)
	return &Ingestor{Store: store, Engine: e}, store, svc
}

func newRouter(ig *Ingestor) http.Handler {
	r := chi.NewRouter()
	r.Post("/webhooks/generic/{key}", ig.Generic)
	r.Post("/webhooks/alertmanager/{key}", ig.Alertmanager)
	r.Post("/webhooks/email-in/{key}", ig.EmailIn)
	return r
}

func TestGeneric_CreatesIncidentAndTriggersEscalation(t *testing.T) {
	ig, store, svc := newTestIngestor(t)
	router := newRouter(ig)

	body := `{"title":"disk full","description":"/var is at 98%"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/generic/"+svc.WebhookKey, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	incidents, err := store.ListIncidents("triggered", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 || incidents[0].Title != "disk full" {
		t.Fatalf("expected 1 triggered incident titled 'disk full', got %+v", incidents)
	}
	if !incidents[0].NextEscalationAt.Valid {
		t.Error("expected escalation to have been triggered on creation")
	}
}

func TestGeneric_UnknownKeyReturns404(t *testing.T) {
	ig, _, _ := newTestIngestor(t)
	router := newRouter(ig)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/generic/does-not-exist", bytes.NewBufferString(`{"title":"x"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown webhook key, got %d", rec.Code)
	}
}

func TestGeneric_MissingTitleReturns400(t *testing.T) {
	ig, _, svc := newTestIngestor(t)
	router := newRouter(ig)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/generic/"+svc.WebhookKey, bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing title, got %d", rec.Code)
	}
}

func TestGeneric_DedupKeyPreventsSecondIncident(t *testing.T) {
	ig, store, svc := newTestIngestor(t)
	router := newRouter(ig)

	body := `{"title":"disk full","dedup_key":"disk-full-host1"}`
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/generic/"+svc.WebhookKey, bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("request %d: unexpected status %d", i, rec.Code)
		}
	}

	incidents, err := store.ListIncidents("all", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected exactly 1 incident from 3 re-fires of the same dedup_key, got %d", len(incidents))
	}
}

func TestAlertmanager_FiringThenResolved(t *testing.T) {
	ig, store, svc := newTestIngestor(t)
	router := newRouter(ig)

	firing := `{
		"status": "firing",
		"alerts": [
			{"status":"firing","labels":{"alertname":"HighCPU"},"annotations":{"summary":"CPU at 95%"},"fingerprint":"fp-cpu-1"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager/"+svc.WebhookKey, bytes.NewBufferString(firing))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("firing request: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	incidents, _ := store.ListIncidents("triggered", 10)
	if len(incidents) != 1 || incidents[0].Title != "HighCPU" {
		t.Fatalf("expected 1 triggered HighCPU incident, got %+v", incidents)
	}

	resolved := `{
		"status": "resolved",
		"alerts": [
			{"status":"resolved","labels":{"alertname":"HighCPU"},"fingerprint":"fp-cpu-1"}
		]
	}`
	req = httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager/"+svc.WebhookKey, bytes.NewBufferString(resolved))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolved request: expected 200, got %d", rec.Code)
	}

	got, err := store.IncidentByID(incidents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "resolved" {
		t.Errorf("expected incident auto-resolved by Alertmanager, got status=%s", got.Status)
	}
}

func TestEmailIn_CreatesIncident(t *testing.T) {
	ig, store, svc := newTestIngestor(t)
	router := newRouter(ig)

	body := `{"From":"ops@example.com","Subject":"Site down","TextBody":"getting 502s everywhere"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/email-in/"+svc.WebhookKey, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	incidents, err := store.ListIncidents("triggered", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 1 || incidents[0].Title != "Site down" {
		t.Fatalf("expected 1 incident titled 'Site down', got %+v", incidents)
	}
}
