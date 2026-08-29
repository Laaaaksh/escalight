package httpserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Laaaaksh/escalight/internal/db"
	"github.com/Laaaaksh/escalight/internal/engine"
)

type schedulesListData struct {
	pageData
	Schedules []*db.Schedule
	OnCallNow map[string]string // schedule ID -> on-call user name
}

func (s *Server) schedulesList(w http.ResponseWriter, r *http.Request) {
	schedules, err := s.Store.ListSchedules()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	onCall := map[string]string{}
	for _, sch := range schedules {
		name, err := s.onCallUserName(sch)
		if err == nil && name != "" {
			onCall[sch.ID] = name
		}
	}
	s.render(w, r, "schedules_list.html", schedulesListData{pageData: s.pageData(r, "schedules"), Schedules: schedules, OnCallNow: onCall})
}

func (s *Server) onCallUserName(sch *db.Schedule) (string, error) {
	now := time.Now().UTC()
	overrides, err := s.Store.OverridesInRange(sch.ID, now.Format(time.RFC3339), now.Add(time.Second).Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	uid, err := engine.OnCallUserID(sch, overrides, now)
	if err != nil || uid == "" {
		return "", err
	}
	u, err := s.Store.UserByID(uid)
	if err != nil {
		return "", err
	}
	return u.Name, nil
}

func (s *Server) scheduleNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "schedule_form_new.html", pageData{User: toUserView(contextUser(r)), Active: "schedules"})
}

func (s *Server) scheduleCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	sch, err := s.Store.CreateSchedule(name, "UTC")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/schedules/"+sch.ID, http.StatusFound)
}

func (s *Server) scheduleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.logErr(s.Store.DeleteSchedule(id), "delete schedule")
	http.Redirect(w, r, "/schedules", http.StatusFound)
}

type calendarDay struct {
	Date     string
	OnCall   string
	Override bool
}

type overrideRow struct {
	*db.ScheduleOverride
	UserName string
}

type scheduleDetailData struct {
	pageData
	Schedule     *db.Schedule
	Users        []*db.User
	UserOrderCSV string
	StartAtLocal string
	Calendar     []calendarDay
	Overrides    []overrideRow
}

func (s *Server) scheduleDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sch, err := s.Store.ScheduleByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	users, err := s.Store.ListUsers()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := scheduleDetailData{pageData: s.pageData(r, "schedules"), Schedule: sch, Users: users}
	if sch.Rotation != nil {
		data.UserOrderCSV = strings.Join(sch.Rotation.UserOrder, ",")
		if t, err := time.Parse(time.RFC3339, sch.Rotation.StartAt); err == nil {
			data.StartAtLocal = t.Format("2006-01-02T15:04")
		}
	}

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -1)
	to := now.AddDate(0, 0, 13)
	overrides, err := s.Store.OverridesInRange(id, from.Format(time.RFC3339), to.Format(time.RFC3339))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	usersByID, _ := s.Store.UsersByIDs(userIDsFromOverrides(overrides, sch))
	for _, o := range overrides {
		name := o.UserID
		if u, ok := usersByID[o.UserID]; ok {
			name = u.Name
		}
		data.Overrides = append(data.Overrides, overrideRow{ScheduleOverride: o, UserName: name})
	}

	if sch.Rotation != nil {
		for d := 0; d < 14; d++ {
			day := now.AddDate(0, 0, d)
			uid, shiftStart, _, err := engine.ShiftAt(sch, day)
			if err != nil {
				continue
			}
			dayOverrides, _ := s.Store.OverridesInRange(id, day.Format(time.RFC3339), day.Add(24*time.Hour).Format(time.RFC3339))
			isOverride := len(dayOverrides) > 0
			if isOverride {
				uid = dayOverrides[0].UserID
			}
			name := uid
			if u, ok := usersByID[uid]; ok {
				name = u.Name
			} else if u, err := s.Store.UserByID(uid); err == nil {
				name = u.Name
			}
			_ = shiftStart
			data.Calendar = append(data.Calendar, calendarDay{Date: day.Format("Mon Jan 2"), OnCall: name, Override: isOverride})
		}
	}

	s.render(w, r, "schedule_detail.html", data)
}

func userIDsFromOverrides(overrides []*db.ScheduleOverride, sch *db.Schedule) []string {
	ids := map[string]bool{}
	for _, o := range overrides {
		ids[o.UserID] = true
	}
	if sch.Rotation != nil {
		for _, uid := range sch.Rotation.UserOrder {
			ids[uid] = true
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out
}

func (s *Server) scheduleSetRotation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = r.ParseForm()
	rotationType := r.FormValue("rotation_type")
	startLocal := r.FormValue("start_at") // "2006-01-02T15:04", interpreted as UTC for v1
	userOrder := r.Form["user_order"]

	t, err := time.Parse("2006-01-02T15:04", startLocal)
	if err != nil {
		http.Redirect(w, r, "/schedules/"+id, http.StatusFound)
		return
	}
	startAt := t.UTC().Format(time.RFC3339)

	if err := s.Store.SetRotation(id, rotationType, "", startAt, userOrder); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/schedules/"+id, http.StatusFound)
}

func (s *Server) scheduleAddOverride(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = r.ParseForm()
	userID := r.FormValue("user_id")
	startLocal := r.FormValue("start_at")
	endLocal := r.FormValue("end_at")

	start, err1 := time.Parse("2006-01-02T15:04", startLocal)
	end, err2 := time.Parse("2006-01-02T15:04", endLocal)
	if err1 != nil || err2 != nil || !end.After(start) {
		http.Redirect(w, r, "/schedules/"+id, http.StatusFound)
		return
	}

	if _, err := s.Store.AddOverride(id, userID, start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/schedules/"+id, http.StatusFound)
}

func (s *Server) scheduleDeleteOverride(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	overrideID := chi.URLParam(r, "overrideID")
	s.logErr(s.Store.DeleteOverride(overrideID), "delete override")
	http.Redirect(w, r, "/schedules/"+id, http.StatusFound)
}
