package httpserver

import (
	"net/http"
	"strings"

	"github.com/Laaaaksh/escalight/internal/auth"
	"github.com/Laaaaksh/escalight/internal/db"
)

type usersListData struct {
	pageData
	Users []*db.User
	Error string
}

func (s *Server) usersList(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "users_list.html", usersListData{pageData: s.pageData(r, "users"), Users: users, Error: r.URL.Query().Get("error")})
}

func (s *Server) userCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	slackID := strings.TrimSpace(r.FormValue("slack_user_id"))
	isAdmin := r.FormValue("is_admin") == "on"

	if name == "" || email == "" || len(password) < 8 {
		http.Redirect(w, r, "/users?error=Name,+email,+and+an+8%2Bchar+password+are+required", http.StatusFound)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	user, err := s.Store.CreateUser(email, name, hash, isAdmin)
	if err != nil {
		http.Redirect(w, r, "/users?error=Could+not+create+user+(email+already+in+use%3F)", http.StatusFound)
		return
	}
	if slackID != "" {
		s.logErr(s.Store.UpdateUserSlackID(user.ID, slackID), "set slack user id for new user")
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}
