package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
)

type incidentRow struct {
	*db.Incident
	ServiceName string
}

type incidentsListData struct {
	pageData
	Incidents []incidentRow
	Status    string
}

func (s *Server) incidentsList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "triggered"
	}
	incidents, err := s.Store.ListIncidents(status, 100)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]incidentRow, 0, len(incidents))
	for _, inc := range incidents {
		name := inc.ServiceID
		if svc, err := s.Store.ServiceByID(inc.ServiceID); err == nil {
			name = svc.Name
		}
		rows = append(rows, incidentRow{Incident: inc, ServiceName: name})
	}

	s.render(w, r, "incidents_list.html", incidentsListData{
		pageData:  s.pageData(r, "incidents"),
		Incidents: rows,
		Status:    status,
	})
}

type incidentDetailData struct {
	pageData
	Incident    *db.Incident
	ServiceName string
	Events      []eventRow
}

type eventRow struct {
	*db.IncidentEvent
}

func (s *Server) incidentDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inc, err := s.Store.IncidentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	svc, err := s.Store.ServiceByID(inc.ServiceID)
	svcName := inc.ServiceID
	if err == nil {
		svcName = svc.Name
	}
	events, err := s.Store.EventsForIncident(id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows := make([]eventRow, 0, len(events))
	for _, e := range events {
		rows = append(rows, eventRow{e})
	}

	s.render(w, r, "incident_detail.html", incidentDetailData{
		pageData:    s.pageData(r, "incidents"),
		Incident:    inc,
		ServiceName: svcName,
		Events:      rows,
	})
}

func (s *Server) incidentAcknowledge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := contextUser(r)
	if err := s.Store.AcknowledgeIncident(id, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.logErr(s.Store.AddEvent(id, "acknowledged", user.Name, ""), "log acknowledged event")
	http.Redirect(w, r, "/incidents/"+id, http.StatusFound)
}

func (s *Server) incidentResolve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := contextUser(r)
	if err := s.Store.ResolveIncident(id, user.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.logErr(s.Store.AddEvent(id, "resolved", user.Name, ""), "log resolved event")
	http.Redirect(w, r, "/incidents/"+id, http.StatusFound)
}

func (s *Server) pageData(r *http.Request, active string) pageData {
	return pageData{User: toUserView(contextUser(r)), Active: active}
}
