// Package utils 提供通用工具，邮件发送配置经 InitMail 注入。
package utils

import (
	"gopkg.in/gomail.v2"

	"GopherCPP/internal/config"
	"GopherCPP/internal/zlog"
)

var mailCfg config.MailConfig

// InitMail 注入 SMTP 配置，须在发信前调用。
func InitMail(cfg config.MailConfig) {
	mailCfg = cfg
}

// SendMail 向指定邮箱发送验证码邮件。
func SendMail(email, code string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", mailCfg.ServerMail)
	m.SetHeader("To", email)
	m.SetHeader("Subject", "GopherCPP 验证码")
	m.SetBody("text/html", "<h1>验证码</h1><p>你的验证码是 "+code+"，有效期 1 分钟。</p>")

	d := gomail.NewDialer(mailCfg.Host, mailCfg.Port, mailCfg.ServerMail, mailCfg.Key)
	d.SSL = true
	if err := d.DialAndSend(m); err != nil {
		zlog.Error("发送邮件失败", "to", email, "err", err)
		return err
	}
	return nil
}
