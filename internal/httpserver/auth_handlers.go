package httpserver

import (
	"net/http"
	"strings"

	"github.com/Laaaaksh/escalight/internal/auth"
)

type loginPageData struct {
	pageData
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if n, err := s.Store.CountUsers(); err == nil && n == 0 {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	s.render(w, r, "login.html", loginPageData{pageData{Error: r.URL.Query().Get("error")}})
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")

	user, err := s.Store.UserByEmail(email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, password) {
		http.Redirect(w, r, "/login?error=Invalid+email+or+password", http.StatusFound)
		return
	}

	token, _, err := s.Store.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, token, r.TLS != nil)
	http.Redirect(w, r, "/incidents", http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.logErr(s.Store.DeleteSession(cookie.Value), "delete session")
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

type setupPageData struct {
	pageData
}

func (s *Server) setupForm(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.CountUsers()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if n > 0 {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	s.render(w, r, "setup.html", setupPageData{pageData{Error: r.URL.Query().Get("error")}})
}

func (s *Server) setupSubmit(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.CountUsers()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if n > 0 {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	_ = r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")

	if name == "" || email == "" || len(password) < 8 {
		http.Redirect(w, r, "/setup?error=Name,+email,+and+an+8%2Bchar+password+are+required", http.StatusFound)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	user, err := s.Store.CreateUser(email, name, hash, true)
	if err != nil {
		http.Redirect(w, r, "/setup?error=Could+not+create+account+(email+already+in+use%3F)", http.StatusFound)
		return
	}

	token, _, err := s.Store.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, token, r.TLS != nil)
	http.Redirect(w, r, "/incidents", http.StatusFound)
}
