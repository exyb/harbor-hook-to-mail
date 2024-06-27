package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	. "github.com/exyb/harbor-hook-to-mail/config"
	. "github.com/exyb/harbor-hook-to-mail/utils"
)

var (
	mailInstance *EmailSender
	config       *MailConfig
	once         sync.Once
)

func GetMailConfig() *MailConfig {
	if config != nil {
		return config
	}
	configPath := os.Getenv("config_file_path")
	config, _ = LoadEmailConfig(configPath)
	return config
}

func GetMailSender(config *MailConfig) *EmailSender {
	if mailInstance != nil {
		return mailInstance
	}
	once.Do(func() {
		// 初始化 instance

		// Decrypt AES encrypted password
		encryptedPassword, err := base64.StdEncoding.DecodeString(config.Email.Sender.Password)
		if err != nil {
			log.Fatalf("Failed to decode mail password from base64: %v", err)
		}

		decryptedPassword, err := DecryptAES(encryptedPassword)
		if err != nil {
			log.Fatalf("Failed to decode mail password: %v", err)
		}

		config.Email.Sender.Password = string(decryptedPassword)

		mailInstance = NewEmailSender(config.Email.Server, config.Email.Port, config.Email.Sender.Address, config.Email.Sender.Password)
	})
	return mailInstance
}

func MailHandler(appName string, mailBodyFile string, attachments []string) error {
	// 假设这里有处理逻辑
	fmt.Println("Reading mail content from:", mailBodyFile)
	mailBody, err := os.ReadFile(mailBodyFile)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return err
	}

	config := GetMailConfig()
	sender := GetMailSender(config)

	// // 发送简单文本邮件
	// err = sender.SendEmail([]string{config.Email.Receiver.Address}, config.Email.Body.Subject, config.Email.Body.Message)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// 发送带附件的邮件
	err = sender.SendEmailWithAttachment(fmt.Sprintf(config.Email.Body.Subject, appName), config.Email.Receiver, config.Email.CC, string(mailBody), attachments)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Email sent successfully!")
	return nil
}

func SendWarnEmail(appName string) error {
	config := GetMailConfig()

	sender := GetMailSender(config)
	mailTitle := fmt.Sprintf("Jenkins interval inform for %s: Warning - hook received, but no key info for %s", time.Now().Format("2006-01-02"), appName)
	// 发送简单文本邮件
	err := sender.SendEmail(config.Email.Receiver, mailTitle, "")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Warning email for %s sent successfully!", appName)
	return nil
}

func SendSuccessEmail(appName string) error {
	config := GetMailConfig()

	sender := GetMailSender(config)
	mailTitle := fmt.Sprintf("Jenkins interval inform for %s: Success - hook received for %s", time.Now().Format("2006-01-02"), appName)
	// 发送简单文本邮件
	err := sender.SendEmail(config.Email.Receiver, mailTitle, "")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Success email for %s sent successfully!\n", appName)
	return nil
}

func SendFailEmail(appName string) error {
	config := GetMailConfig()

	sender := GetMailSender(config)
	mailTitle := fmt.Sprintf("Jenkins interval inform of %s: Failure - no successful build for %s", time.Now().Format("2006-01-02"), appName)
	// 发送简单文本邮件
	err := sender.SendEmail(config.Email.Receiver, mailTitle, "")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Failure email for %s sent successfully!", appName)
	return nil
}
