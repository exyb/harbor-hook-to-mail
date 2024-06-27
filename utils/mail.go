package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/jordan-wright/email"
)

type EmailSender struct {
	Host     string
	Port     int
	Username string
	Password string
}

func NewEmailSender(host string, port int, username, password string) *EmailSender {
	return &EmailSender{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	}
}

func retry(attempts int, sleep time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		fmt.Printf("Attempt %d failed: %v\n", i+1, err)
		if i < attempts-1 {
			time.Sleep(sleep)
		}
	}
	return fmt.Errorf("after %d attempts, last error: %w", attempts, err)
}

func (sender *EmailSender) SendEmail(to []string, subject, text string) error {
	e := email.NewEmail()
	// e.From = fmt.Sprintf("%s <%s>", sender.Username, sender.Username)
	e.From = sender.Username
	e.To = to
	e.Subject = subject
	e.Text = []byte(text)

	addr := fmt.Sprintf("%s:%d", sender.Host, sender.Port)
	auth := smtp.PlainAuth("", sender.Username, sender.Password, sender.Host)

	sendFunc := func() error {
		fmt.Println("Calling sendFunc for SendEmail")
		return e.Send(addr, auth)
	}

	return retry(3, 2*time.Second, sendFunc)
}

func (sender *EmailSender) SendEmailWithAttachment(subject string, to []string, cc []string, mailBody string, mailBodyType string, attachments []string) error {
	e := email.NewEmail()
	// e.From = fmt.Sprintf("%s <%s>", sender.Username, sender.Username)
	e.From = sender.Username
	e.Subject = subject
	e.To = to
	e.Cc = cc
	if strings.ToUpper(mailBodyType) == "HTML" {
		e.HTML = []byte(mailBody)
	}
	if strings.ToUpper(mailBodyType) == "TEXT" {
		e.Text = []byte(mailBody)
	}

	for _, attachment := range attachments {
		_, err := e.AttachFile(attachment)
		if err != nil {
			return err
		}
	}

	addr := fmt.Sprintf("%s:%d", sender.Host, sender.Port)
	auth := smtp.PlainAuth("", sender.Username, sender.Password, sender.Host)

	sendFunc := func() error {
		fmt.Println("Calling sendFunc for SendEmailWithAttachment")
		return e.Send(addr, auth)
	}

	return retry(3, 2*time.Second, sendFunc)
}

func (sender *EmailSender) SendEmailTLS(to []string, subject, text string) error {
	e := email.NewEmail()
	// e.From = fmt.Sprintf("%s <%s>", sender.Username, sender.Username)
	e.From = sender.Username
	e.To = to
	e.Subject = subject
	e.Text = []byte(text)

	addr := fmt.Sprintf("%s:%d", sender.Host, sender.Port)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         sender.Host,
	}

	sendFunc := func() error {
		fmt.Println("Calling sendFunc for SendEmailTLS")
		return e.SendWithTLS(addr, smtp.PlainAuth("", sender.Username, sender.Password, sender.Host), tlsConfig)
	}

	return retry(3, 2*time.Second, sendFunc)
}
