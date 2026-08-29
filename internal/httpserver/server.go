// Package httpserver is Escalight's web UI and JSON API: server-rendered
// HTML pages enhanced with htmx, plus the small JSON endpoints the service
// worker and Slack integration call.
package httpserver

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Laaaaksh/escalight/internal/config"
	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/engine"
	"github.com/Laaaaksh/escalight/internal/notify"
	"github.com/Laaaaksh/escalight/internal/webhooks"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	Store     *db.Store
	Engine    *engine.Engine
	Notify    *notify.Dispatcher
	Ingestor  *webhooks.Ingestor
	Config    config.Config
	VAPIDPub  string
	logger    *slog.Logger
	templates *templateSet
}

func New(store *db.Store, eng *engine.Engine, dispatcher *notify.Dispatcher, ingestor *webhooks.Ingestor, cfg config.Config, vapidPub string, logger *slog.Logger) (*Server, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	return &Server{
		Store: store, Engine: eng, Notify: dispatcher, Ingestor: ingestor,
		Config: cfg, VAPIDPub: vapidPub, logger: logger, templates: tmpl,
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// Deliberately no middleware.RealIP: it trusts X-Forwarded-For/X-Real-IP
	// unconditionally, which is spoofable unless a reverse proxy is known to
	// set and sanitize that header. Self-hosters fronting Escalight with a
	// trusted proxy can add IP-aware middleware themselves.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	r.Get("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticSub, "manifest.json")
	})
	r.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, staticSub, "sw.js")
	})

	// Alert ingestion: authenticated by the per-service key in the URL, not a session.
	r.Post("/webhooks/generic/{key}", s.Ingestor.Generic)
	r.Post("/webhooks/alertmanager/{key}", s.Ingestor.Alertmanager)
	r.Post("/webhooks/email-in/{key}", s.Ingestor.EmailIn)

	// Slack: authenticated by its own request signature, not a session.
	r.Post("/integrations/slack/commands", s.slackCommand)

	r.Get("/login", s.loginForm)
	r.Post("/login", s.loginSubmit)
	r.Post("/logout", s.logout)
	r.Get("/setup", s.setupForm)
	r.Post("/setup", s.setupSubmit)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/incidents", http.StatusFound) })

		r.Get("/incidents", s.incidentsList)
		r.Get("/incidents/{id}", s.incidentDetail)
		r.Post("/api/incidents/{id}/acknowledge", s.incidentAcknowledge)
		r.Post("/api/incidents/{id}/resolve", s.incidentResolve)

		r.Get("/policies", s.policiesList)
		r.Get("/policies/new", s.policyNewForm)
		r.Post("/policies", s.policyCreate)
		r.Get("/policies/{id}/edit", s.policyEditForm)
		r.Post("/policies/{id}", s.policyUpdate)
		r.Post("/policies/{id}/delete", s.policyDelete)

		r.Get("/schedules", s.schedulesList)
		r.Get("/schedules/new", s.scheduleNewForm)
		r.Post("/schedules", s.scheduleCreate)
		r.Get("/schedules/{id}", s.scheduleDetail)
		r.Post("/schedules/{id}/delete", s.scheduleDelete)
		r.Post("/schedules/{id}/rotation", s.scheduleSetRotation)
		r.Post("/schedules/{id}/overrides", s.scheduleAddOverride)
		r.Post("/schedules/{id}/overrides/{overrideID}/delete", s.scheduleDeleteOverride)

		r.Get("/services", s.servicesList)
		r.Get("/services/new", s.serviceNewForm)
		r.Post("/services", s.serviceCreate)
		r.Post("/services/{id}/delete", s.serviceDelete)

		r.Get("/settings", s.settingsPage)
		r.Post("/settings/slack", s.settingsUpdateSlack)
		r.Post("/api/push/subscribe", s.pushSubscribe)
		r.Post("/api/push/unsubscribe", s.pushUnsubscribe)
		r.Post("/api/push/test", s.pushTest)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/users", s.usersList)
			r.Post("/users", s.userCreate)
		})
	})

	return r
}

// logErr records a best-effort operation's error (one where the request
// already has a sensible response regardless of the outcome, e.g. a delete
// button that redirects either way) instead of silently discarding it.
func (s *Server) logErr(err error, action string) {
	if err != nil && s.logger != nil {
		s.logger.Error(action+" failed", "error", err)
	}
}
