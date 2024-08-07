package handlers

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"regexp"
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

	// 定义正则表达式来匹配 "构建结果: SUCCESS" 或 "构建结果: FAILURE"
	re := regexp.MustCompile(`构建结果: (SUCCESS|FAILURE)`)
	// 查找所有匹配项
	matches := re.FindAllStringSubmatch(string(mailBody), 1)
	var buildResult string
	if len(matches) > 0 {
		if matches[0][1] == "SUCCESS" {
			buildResult = "成功"
		} else if matches[0][1] == "FAILURE" {
			buildResult = "失败"
		}
	} else {
		buildResult = "未知"
	}
	config := GetMailConfig()
	sender := GetMailSender(config)

	// // 发送简单文本邮件
	// err = sender.SendEmail([]string{config.Email.Receiver.Address}, config.Email.Body.Subject, config.Email.Body.Message)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// 发送带附件的邮件
	mailTitle := fmt.Sprintf(config.Email.Body.Subject, appName, time.Now().Format("2006-01-02"), buildResult)

	err = sender.SendEmailWithAttachment(mailTitle, config.Email.Receiver, config.Email.CC, string(mailBody), config.Email.Body.Type, attachments)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Email sent successfully!")
	return nil
}

func SendWarnEmail(appName string) error {
	config := GetMailConfig()

	sender := GetMailSender(config)
	mailTitle := fmt.Sprintf("构建警告定时通知 - %s: 应用 %s 成功构建但是存在报错", time.Now().Format("2006-01-02"), appName)
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
	mailTitle := fmt.Sprintf("构建定时通知 - %s: 应用 %s 成功完成构建", time.Now().Format("2006-01-02"), appName)
	// 发送简单文本邮件
	err := sender.SendEmail(config.Email.Receiver, mailTitle, "请参考构建环境日志进行详细排查")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Success email for %s sent successfully!\n", appName)
	return nil
}

func SendFailEmail(appName string) error {
	config := GetMailConfig()

	sender := GetMailSender(config)
	mailTitle := fmt.Sprintf("构建失败定时通知 - %s: 应用 %s 没有收到成功构建信息", time.Now().Format("2006-01-02"), appName)
	// 发送简单文本邮件
	err := sender.SendEmail(config.Email.Receiver, mailTitle, "请结合前序定时通知邮件和当天首封详情邮件, 并参考构建环境日志进行排查")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Failure email for %s sent successfully!", appName)
	return nil
}
