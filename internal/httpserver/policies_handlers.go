package httpserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
)

type policiesListData struct {
	pageData
	Policies []*db.EscalationPolicy
}

func (s *Server) policiesList(w http.ResponseWriter, r *http.Request) {
	policies, err := s.Store.ListPolicies()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "policies_list.html", policiesListData{pageData: s.pageData(r, "policies"), Policies: policies})
}

// targetOption is a <select> entry for choosing a step's notification
// target: a specific person, or "whoever is on call" for a schedule.
type targetOption struct {
	Value string // "user:<id>" or "schedule:<id>"
	Label string
}

type stepFormRow struct {
	WaitMinutes int
	Target      string // selected targetOption.Value
	ViaEmail    bool
	ViaPush     bool
	ViaSlack    bool
	ViaDiscord  bool
}

type policyFormData struct {
	pageData
	Policy  *db.EscalationPolicy
	Steps   []stepFormRow
	Targets []targetOption
	Editing bool
}

func (s *Server) targetOptions() ([]targetOption, error) {
	var opts []targetOption
	users, err := s.Store.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		opts = append(opts, targetOption{Value: "user:" + u.ID, Label: u.Name})
	}
	schedules, err := s.Store.ListSchedules()
	if err != nil {
		return nil, err
	}
	for _, sc := range schedules {
		opts = append(opts, targetOption{Value: "schedule:" + sc.ID, Label: "On-call: " + sc.Name})
	}
	return opts, nil
}

func (s *Server) policyNewForm(w http.ResponseWriter, r *http.Request) {
	opts, err := s.targetOptions()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "policy_form.html", policyFormData{
		pageData: s.pageData(r, "policies"),
		Policy:   &db.EscalationPolicy{},
		Steps:    []stepFormRow{{WaitMinutes: 5, ViaEmail: true, ViaPush: true}},
		Targets:  opts,
	})
}

func (s *Server) policyEditForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	policy, err := s.Store.PolicyByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	steps, err := s.Store.StepsForPolicy(id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	opts, err := s.targetOptions()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]stepFormRow, 0, len(steps))
	for _, st := range steps {
		row := stepFormRow{WaitMinutes: st.WaitMinutes}
		if len(st.Targets) > 0 {
			t := st.Targets[0]
			row.Target = t.TargetType + ":" + t.TargetID
			row.ViaEmail, row.ViaPush, row.ViaSlack, row.ViaDiscord = t.ViaEmail, t.ViaPush, t.ViaSlack, t.ViaDiscord
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		rows = []stepFormRow{{WaitMinutes: 5, ViaEmail: true, ViaPush: true}}
	}

	s.render(w, r, "policy_form.html", policyFormData{
		pageData: s.pageData(r, "policies"),
		Policy:   policy,
		Steps:    rows,
		Targets:  opts,
		Editing:  true,
	})
}

func (s *Server) policyCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	repeat, _ := strconv.Atoi(r.FormValue("repeat"))

	policy, err := s.Store.CreatePolicy(name, description, repeat)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.Store.ReplaceSteps(policy.ID, parseStepsForm(r)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/policies", http.StatusFound)
}

func (s *Server) policyUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	repeat, _ := strconv.Atoi(r.FormValue("repeat"))

	if _, err := s.Store.DB.Exec(`UPDATE escalation_policies SET name = ?, description = ?, repeat = ? WHERE id = ?`, name, description, repeat, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := s.Store.ReplaceSteps(id, parseStepsForm(r)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/policies", http.StatusFound)
}

func (s *Server) policyDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.logErr(s.Store.DeletePolicy(id), "delete policy")
	http.Redirect(w, r, "/policies", http.StatusFound)
}

// parseStepsForm reads the dynamically-added step rows the policy form's
// inline JS names step_0_wait, step_0_target, step_0_via_email, step_1_wait, ...
func parseStepsForm(r *http.Request) []db.EscalationStep {
	var steps []db.EscalationStep
	for i := 0; ; i++ {
		prefix := "step_" + strconv.Itoa(i) + "_"
		waitStr, ok := r.Form[prefix+"wait"]
		if !ok || len(waitStr) == 0 {
			break
		}
		wait, _ := strconv.Atoi(waitStr[0])
		target := r.FormValue(prefix + "target")
		targetType, targetID, ok := strings.Cut(target, ":")
		if !ok {
			continue
		}
		steps = append(steps, db.EscalationStep{
			WaitMinutes: wait,
			Targets: []db.EscalationStepTarget{{
				TargetType: targetType,
				TargetID:   targetID,
				ViaEmail:   r.FormValue(prefix+"via_email") == "on",
				ViaPush:    r.FormValue(prefix+"via_push") == "on",
				ViaSlack:   r.FormValue(prefix+"via_slack") == "on",
				ViaDiscord: r.FormValue(prefix+"via_discord") == "on",
			}},
		})
	}
	return steps
}
