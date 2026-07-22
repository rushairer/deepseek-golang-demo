package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	SMTPHost, SMTPPort, SMTPUser, SMTPPass, SMTPFrom string
	SMTPStartTLS                                     bool
	EmailTargets, WebhookTargets                     map[string]string
	AllowHTTPWebhooks, AllowPrivateWebhooks          bool
	Timeout                                          time.Duration
}
type Sender struct {
	config Config
	client *http.Client
}

func NewSender(c Config) *Sender {
	s := &Sender{config: c}
	s.client = &http.Client{Timeout: c.Timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many webhook redirects")
		}
		return s.validateWebhookURL(r.Context(), r.URL)
	}}
	return s
}
func (s *Sender) HasTarget(channel, target string) bool {
	switch strings.ToLower(channel) {
	case "email":
		_, ok := s.config.EmailTargets[target]
		return ok
	case "webhook":
		_, ok := s.config.WebhookTargets[target]
		return ok
	}
	return false
}
func (s *Sender) Send(ctx context.Context, channel, target, message string) error {
	switch strings.ToLower(channel) {
	case "email":
		to, ok := s.config.EmailTargets[target]
		if !ok {
			return fmt.Errorf("email target not configured")
		}
		return s.sendEmail(ctx, to, message)
	case "webhook":
		u, ok := s.config.WebhookTargets[target]
		if !ok {
			return fmt.Errorf("webhook target not configured")
		}
		return s.sendWebhook(ctx, u, message)
	}
	return fmt.Errorf("unsupported notification channel")
}
func (s *Sender) sendEmail(ctx context.Context, to, message string) error {
	if s.config.SMTPHost == "" || s.config.SMTPFrom == "" {
		return fmt.Errorf("SMTP_HOST and SMTP_FROM are required")
	}
	address := net.JoinHostPort(s.config.SMTPHost, s.config.SMTPPort)
	conn, err := (&net.Dialer{Timeout: s.config.Timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(s.config.Timeout))
	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if s.config.SMTPStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not advertise STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.config.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.config.SMTPUser != "" {
		if err := client.Auth(smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPass, s.config.SMTPHost)); err != nil {
			return err
		}
	}
	if err := client.Mail(s.config.SMTPFrom); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	payload := fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: DeepSeek Agent Notification\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", to, s.config.SMTPFrom, message)
	if _, err := w.Write([]byte(payload)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
func (s *Sender) sendWebhook(ctx context.Context, raw, message string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if err := s.validateWebhookURL(ctx, u); err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"message": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "deepseek-golang-agent/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
func (s *Sender) validateWebhookURL(ctx context.Context, u *url.URL) error {
	if u.User != nil {
		return fmt.Errorf("webhook URL must not contain user info")
	}
	if u.Scheme != "https" && !(s.config.AllowHTTPWebhooks && u.Scheme == "http") {
		return fmt.Errorf("webhook URL must use HTTPS")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL host is required")
	}
	if s.config.AllowPrivateWebhooks {
		return nil
	}
	lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(lookup, host)
	if err != nil {
		return err
	}
	for _, a := range ips {
		if isPrivateOrLocal(a.IP) {
			return fmt.Errorf("webhook host resolves to a private or local address")
		}
	}
	return nil
}
func isPrivateOrLocal(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}
