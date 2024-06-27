package tests

import (
	"log"
	"os"
	"testing"

	. "github.com/exyb/harbor-hook-to-mail/config"
	. "github.com/exyb/harbor-hook-to-mail/utils"
)

func TestSMTPClient_SendEmail(t *testing.T) {
	configPath := os.Getenv("config_file_path")
	config, err := LoadEmailConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	sender := NewEmailSender(config.Email.Server, config.Email.Port, config.Email.Sender.Address, config.Email.Sender.Password)

	err = sender.SendEmail(config.Email.Receiver, config.Email.Body.Subject, config.Email.Body.Message)
	if err != nil {
		log.Fatal(err)
	}

	// err = sender.SendEmailWithAttachment([]string{config.Email.Receiver.Address}, config.Email.Body.Subject, config.Email.Body.Message, config.Email.Attachments)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	t.Logf("Email sent successfully!")
}
