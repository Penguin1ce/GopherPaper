package utils

//import (
//	"testing"
//
//	"GopherCPP/internal/config"
//)
//
//const testConfigPath = "../../config/config.toml"
//
//func loadMailConfig(t *testing.T) config.MailConfig {
//	t.Helper()
//
//	cfg, err := config.Load(testConfigPath)
//	if err != nil {
//		t.Skipf("跳过：读取配置失败 %v", err)
//	}
//
//	mail := cfg.Mail
//	if mail.ServerMail == "" || mail.Host == "" || mail.Port == 0 || mail.Key == "" {
//		t.Skip("跳过：SMTP 邮件配置不完整")
//	}
//	if mail.RecipientMail == "" {
//		t.Skip("跳过：未配置 mail.recipient_mail")
//	}
//	return mail
//}
//
//func TestSendMail_FromConfig(t *testing.T) {
//	mail := loadMailConfig(t)
//	InitMail(mail)
//
//	const code = "123456"
//	if err := SendMail(mail.RecipientMail, code); err != nil {
//		t.Fatalf("发送验证码邮件失败 %v", err)
//	}
//	t.Logf("验证码邮件已发送至 %s", mail.RecipientMail)
//}
