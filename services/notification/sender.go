package notification

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
)

// Send 发送通知
func Send(channel string, message string, params map[string]interface{}) error {
	switch channel {
	case "email":
		return sendEmail(message, params)
	case "sms":
		return sendSMS(message, params)
	case "webhook":
		return sendWebhook(message, params)
	default:
		return fmt.Errorf("未知的通知渠道: %s", channel)
	}
}

// sendEmail 发送邮件通知
func sendEmail(message string, params map[string]interface{}) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	to, ok := params["to"].(string)
	if !ok {
		return fmt.Errorf("无效的收件人参数")
	}

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	msg := fmt.Sprintf("To: %s\r\nSubject: 系统通知\r\n\r\n%s", to, message)

	if err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{to}, []byte(msg)); err != nil {
		log.Printf("发送邮件失败: %v", err)
		return err
	}

	return nil
}

// sendSMS 发送短信通知
func sendSMS(message string, params map[string]interface{}) error {
	// 这里需要集成具体的短信服务商API
	log.Printf("发送短信通知: %s", message)
	return nil
}

// sendWebhook 发送Webhook通知
func sendWebhook(message string, params map[string]interface{}) error {
	url, ok := params["url"].(string)
	if !ok {
		return fmt.Errorf("无效的Webhook URL参数")
	}

	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		log.Printf("发送Webhook通知失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Webhook请求失败，状态码: %d", resp.StatusCode)
	}

	return nil
}
