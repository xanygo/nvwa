//  Copyright(C) 2026 github.com/hidu  All Rights Reserved.
//  Author: hidu <duv123+git@gmail.com>
//  Date: 2026-03-15

package nvwa

import "github.com/xanygo/anygo/xi18n"

var i18nResource = &xi18n.Bundle{}

func init() {
	en := i18nResource.MustLocalize(xi18n.LangEn)
	en.MustAdd("user",
		&xi18n.Message{
			Key:   "loginTitle",
			Other: "Login",
		},
		&xi18n.Message{
			Key:   "err400",
			Other: "invalid request",
		},
		&xi18n.Message{
			Key:   "loginFailed",
			Other: "login failed",
		},
		&xi18n.Message{
			Key:   "loginSuc",
			Other: "login success",
		},
		&xi18n.Message{
			Key:   "invalidCode",
			Other: "invalid caption",
		},
	)
	zh := i18nResource.MustLocalize(xi18n.LangZh)
	zh.MustAdd("user",
		&xi18n.Message{
			Key:   "loginTitle",
			Other: "登录认证",
		},
		&xi18n.Message{
			Key:   "err400",
			Other: "非法请求",
		},
		&xi18n.Message{
			Key:   "loginFailed",
			Other: "登录失败",
		},
		&xi18n.Message{
			Key:   "loginSuc",
			Other: "登录成功",
		},
		&xi18n.Message{
			Key:   "invalidCode",
			Other: "验证码错误",
		},
	)
}
