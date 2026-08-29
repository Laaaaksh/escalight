package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Laaaaksh/escalight/internal/config"
	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/engine"
	"github.com/Laaaaksh/escalight/internal/notify"
	"github.com/Laaaaksh/escalight/internal/webhooks"
)

func newTestServer(t *testing.T) (*httptest.Server, *db.Store) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	store := db.NewStore(conn)

	dispatcher := &notify.Dispatcher{
		Store: store, BaseURL: "http://test",
		Email: &notify.EmailSender{}, Slack: &notify.SlackSender{}, Discord: &notify.DiscordSender{}, Push: &notify.WebPushSender{},
	}
	eng := engine.New(store, dispatcher, nil)
	ingestor := &webhooks.Ingestor{Store: store, Engine: eng}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv, err := New(store, eng, dispatcher, ingestor, config.Config{BaseURL: "http://test"}, "test-vapid-pub", logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	srv.Config.BaseURL = ts.URL
	return ts, store
}

// newClient returns an http.Client with a cookie jar (so session cookies
// persist across requests) that does not follow redirects, so tests can
// assert on the redirect itself.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestUnauthenticatedRequestRedirectsToLogin(t *testing.T) {
	ts, _ := newTestServer(t)
	client := newClient(t)

	resp, err := client.Get(ts.URL + "/incidents")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/login" {
		t.Errorf("expected 302 to /login, got %d Location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestSetupThenLoginFlow(t *testing.T) {
	ts, store := newTestServer(t)
	client := newClient(t)

	// No users yet: /login itself redirects to /setup.
	resp, err := client.Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/setup" {
		t.Errorf("expected redirect to /setup with no users, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	form := url.Values{"name": {"Alice"}, "email": {"alice@example.com"}, "password": {"correcthorsebattery"}}
	resp, err = client.PostForm(ts.URL+"/setup", form)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/incidents" {
		t.Fatalf("expected setup to redirect to /incidents, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	n, _ := store.CountUsers()
	if n != 1 {
		t.Fatalf("expected 1 user after setup, got %d", n)
	}

	// Authenticated now: /incidents should load directly.
	resp, err = client.Get(ts.URL + "/incidents")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for authenticated /incidents, got %d", resp.StatusCode)
	}

	// A second /setup attempt must be rejected now that a user exists.
	resp, err = client.Get(ts.URL + "/setup")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/login" {
		t.Errorf("expected /setup to redirect to /login once a user exists, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	ts, _ := newTestServer(t)
	setupClient := newClient(t)
	setupClient.PostForm(ts.URL+"/setup", url.Values{"name": {"Alice"}, "email": {"alice@example.com"}, "password": {"correcthorsebattery"}})

	client := newClient(t)
	resp, err := client.PostForm(ts.URL+"/login", url.Values{"email": {"alice@example.com"}, "password": {"wrong"}})
	if err != nil {
		t.Fatal(err)
	}
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusFound || !strings.HasPrefix(loc, "/login?error=") {
		t.Errorf("expected redirect back to /login with an error, got %d %q", resp.StatusCode, loc)
	}

	// No session cookie should have been set for a failed login.
	resp2, err := client.Get(ts.URL + "/incidents")
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusFound {
		t.Errorf("expected still-unauthenticated client to be redirected, got %d", resp2.StatusCode)
	}
}

func TestFullIncidentLifecycleOverHTTP(t *testing.T) {
	ts, store := newTestServer(t)
	client := newClient(t)
	client.PostForm(ts.URL+"/setup", url.Values{"name": {"Alice"}, "email": {"alice@example.com"}, "password": {"correcthorsebattery"}})

	user, err := store.UserByEmail("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := store.CreatePolicy("Primary", "", 0)
	store.ReplaceSteps(policy.ID, []db.EscalationStep{
		{WaitMinutes: 5, Targets: []db.EscalationStepTarget{{TargetType: "user", TargetID: user.ID, ViaEmail: true}}},
	})
	svc, _ := store.CreateService("API", policy.ID)

	// Fire a generic webhook alert - the same request shape a monitoring tool would send.
	resp, err := client.Post(ts.URL+"/webhooks/generic/"+svc.WebhookKey, "application/json", strings.NewReader(`{"title":"db down"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating incident via webhook, got %d", resp.StatusCode)
	}

	incidents, err := store.ListIncidents("triggered", 10)
	if err != nil || len(incidents) != 1 {
		t.Fatalf("expected 1 triggered incident, got %v (err=%v)", incidents, err)
	}
	incID := incidents[0].ID

	// Acknowledge it via the authenticated HTTP endpoint (the same one the
	// push notification's service worker calls).
	resp, err = client.Post(ts.URL+"/api/incidents/"+incID+"/acknowledge", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 after acknowledge, got %d", resp.StatusCode)
	}

	got, err := store.IncidentByID(incID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "acknowledged" {
		t.Errorf("expected status=acknowledged, got %s", got.Status)
	}

	// Resolve it too.
	resp, err = client.Post(ts.URL+"/api/incidents/"+incID+"/resolve", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 after resolve, got %d", resp.StatusCode)
	}
	got, err = store.IncidentByID(incID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "resolved" {
		t.Errorf("expected status=resolved, got %s", got.Status)
	}
}

func TestNonAdminCannotCreateUsers(t *testing.T) {
	ts, store := newTestServer(t)
	client := newClient(t)
	client.PostForm(ts.URL+"/setup", url.Values{"name": {"Alice"}, "email": {"alice@example.com"}, "password": {"correcthorsebattery"}})

	// Demote alice to a non-admin so /users should refuse her.
	store.DB.Exec(`UPDATE users SET is_admin = 0 WHERE email = 'alice@example.com'`)

	client2 := newClient(t)
	resp, err := client2.PostForm(ts.URL+"/login", url.Values{"email": {"alice@example.com"}, "password": {"correcthorsebattery"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected login to succeed, got %d", resp.StatusCode)
	}

	resp, err = client2.Get(ts.URL + "/users")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin accessing /users, got %d", resp.StatusCode)
	}
}
