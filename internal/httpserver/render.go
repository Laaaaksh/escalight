package httpserver

import (
	"embed"
	"html/template"
	"net/http"
	"time"
)

//go:embed templates/*.html
var templatesFS embed.FS

var funcMap = template.FuncMap{
	"friendlyTime": func(rfc3339 string) string {
		if rfc3339 == "" {
			return ""
		}
		t, err := time.Parse(time.RFC3339, rfc3339)
		if err != nil {
			return rfc3339
		}
		return t.Local().Format("Jan 2, 15:04:05")
	},
	"badgeClass": func(status string) string {
		return "badge-" + status
	},
	"add1": func(i int) int { return i + 1 },
}

// pageNames lists every content template file; each must {{define "content"}}.
var pageNames = []string{
	"login.html", "setup.html",
	"incidents_list.html", "incident_detail.html",
	"policies_list.html", "policy_form.html",
	"schedules_list.html", "schedule_form_new.html", "schedule_detail.html",
	"services_list.html", "service_form.html",
	"users_list.html",
	"settings.html",
}

type templateSet struct {
	pages map[string]*template.Template
}

func loadTemplates() (*templateSet, error) {
	base, err := template.New("layout.html").Funcs(funcMap).ParseFS(templatesFS, "templates/layout.html")
	if err != nil {
		return nil, err
	}

	ts := &templateSet{pages: map[string]*template.Template{}}
	for _, name := range pageNames {
		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		clone, err = clone.ParseFS(templatesFS, "templates/"+name)
		if err != nil {
			return nil, err
		}
		ts.pages[name] = clone
	}
	return ts, nil
}

// pageData is embedded (by convention, as PageData) in every handler's
// template data struct so layout.html has what it needs for nav/auth chrome.
type pageData struct {
	User      *userView
	Active    string // nav section key, matches layout.html's nav
	Error     string
	FirstUser bool
}

type userView struct {
	Name        string
	Email       string
	Admin       bool
	SlackUserID string
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data any) {
	tmpl, ok := s.templates.pages[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		s.logger.Error("render template", "page", page, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
