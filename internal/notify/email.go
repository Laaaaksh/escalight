package notify

import (
	"fmt"
	"net/smtp"
)

type EmailSender struct {
	Host, Port, User, Pass, From string
}

func (e *EmailSender) Configured() bool {
	return e.Host != ""
}

func (e *EmailSender) Send(to, subject, body string) error {
	if !e.Configured() {
		return fmt.Errorf("email not configured (set ESCALIGHT_SMTP_HOST)")
	}

	addr := fmt.Sprintf("%s:%s", e.Host, e.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", e.From, to, subject, body)

	var auth smtp.Auth
	if e.User != "" {
		auth = smtp.PlainAuth("", e.User, e.Pass, e.Host)
	}
	return smtp.SendMail(addr, auth, e.From, []string{to}, []byte(msg))
}
