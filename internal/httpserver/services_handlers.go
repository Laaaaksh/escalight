package httpserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
)

type serviceRow struct {
	*db.Service
	PolicyName      string
	GenericURL      string
	AlertmanagerURL string
	EmailInURL      string
}

type servicesListData struct {
	pageData
	Services []serviceRow
}

func (s *Server) servicesList(w http.ResponseWriter, r *http.Request) {
	services, err := s.Store.ListServices()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows := make([]serviceRow, 0, len(services))
	for _, svc := range services {
		policyName := svc.EscalationPolicyID
		if p, err := s.Store.PolicyByID(svc.EscalationPolicyID); err == nil {
			policyName = p.Name
		}
		rows = append(rows, serviceRow{
			Service:         svc,
			PolicyName:      policyName,
			GenericURL:      fmt.Sprintf("%s/webhooks/generic/%s", s.Config.BaseURL, svc.WebhookKey),
			AlertmanagerURL: fmt.Sprintf("%s/webhooks/alertmanager/%s", s.Config.BaseURL, svc.WebhookKey),
			EmailInURL:      fmt.Sprintf("%s/webhooks/email-in/%s", s.Config.BaseURL, svc.WebhookKey),
		})
	}
	s.render(w, r, "services_list.html", servicesListData{pageData: s.pageData(r, "services"), Services: rows})
}

type serviceFormData struct {
	pageData
	Policies []*db.EscalationPolicy
}

func (s *Server) serviceNewForm(w http.ResponseWriter, r *http.Request) {
	policies, err := s.Store.ListPolicies()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "service_form.html", serviceFormData{pageData: s.pageData(r, "services"), Policies: policies})
}

func (s *Server) serviceCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	policyID := r.FormValue("policy_id")
	if name == "" || policyID == "" {
		http.Redirect(w, r, "/services/new", http.StatusFound)
		return
	}
	if _, err := s.Store.CreateService(name, policyID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/services", http.StatusFound)
}

func (s *Server) serviceDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.logErr(s.Store.DeleteService(id), "delete service")
	http.Redirect(w, r, "/services", http.StatusFound)
}
