//  Copyright(C) 2026 github.com/hidu  All Rights Reserved.
//  Author: hidu <duv123+git@gmail.com>
//  Date: 2026-03-14

package nvwa

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/xanygo/anygo"
	"github.com/xanygo/anygo/ds/xctx"
	"github.com/xanygo/anygo/ds/xhash"
	"github.com/xanygo/anygo/store/xsession"
	"github.com/xanygo/anygo/xhttp"
	"github.com/xanygo/anygo/xi18n"
	"github.com/xanygo/anygo/xlog"
	"github.com/xanygo/webr"
)

var ctxUserKey = xctx.NewKey()

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxUserKey, user)
}

func UserFormContext(ctx context.Context) *User {
	user, _ := ctx.Value(ctxUserKey).(*User)
	return user
}

type User struct {
	Username string
	AuthCode string // 登录校验码，由 Password + Salt 等计算而得
	Disabled bool
	Raw      any
}

// 固定值，Client 判断有这个cookie则认为是已经登录的
const authCookieName = "auth"

type userHandler struct {
	Board *Dashboard
}

func (t *userHandler) loadUserFromCooke(r *http.Request) (*User, error) {
	ck, err := r.Cookie(authCookieName)
	var name, token string
	if err == nil {
		name, token, err = t.parserAuthCookieValue(ck.Value)
	}
	var user *User
	if err == nil {
		user, err = t.Board.UserFinder(r.Context(), name)
	}
	if err == nil {
		var want string
		want, err = t.authCookieValue(user)
		if err == nil && want == token {
			err = errors.New("invalid auth token")
		}
	}
	return user, err
}

func (t *userHandler) authCookieValue(u *User) (string, error) {
	m5 := xhash.Md5(u.Username + "--" + u.AuthCode)
	return t.Board.encrypt(u.Username + "|" + m5)
}

func (t *userHandler) parserAuthCookieValue(str string) (name string, token string, err error) {
	txt, err := t.Board.decrypt(str)
	if err != nil {
		return "", "", err
	}
	var ok bool
	name, token, ok = strings.Cut(txt, "|")
	if !ok {
		return "", "", errors.New("invalid auth cookie")
	}
	return name, token, nil
}

func (t *userHandler) checkLogin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := t.loadUserFromCooke(r)
		if err != nil {
			xlog.AddAttr(r.Context(), xlog.ErrorAttr("error", err))
			http.Redirect(w, r, xhttp.PathJoin(t.Board.GetPathPrefix(), "/login"), http.StatusFound)
			return
		}
		ctx := ContextWithUser(r.Context(), user)
		xlog.AddMetaAttr(ctx, xlog.String("user", user.Username))

		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

func (t *userHandler) loginIndex(w http.ResponseWriter, req *http.Request) {
	values := map[string]any{
		"Title": anygo.Must1(xi18n.RC(i18nResource, req, "user/loginTitle")),
	}
	t.Board.Render(req.Context(), w, req, "login.html", values)
}

type loginForm struct {
	Name     string `validator:"required"`
	Password string `validator:"required"`
	Code     string `validator:"required"`
	TP       string `validator:"required"`
}

func (t *userHandler) loginCheck(w http.ResponseWriter, req *http.Request) {
	form := &loginForm{}
	if err := xhttp.Bind(req, form); err != nil {
		webr.WriteJSON(w, 400, "bad request:"+err.Error(), nil)
		return
	}
	ctx := req.Context()

	session := xsession.FromContext(ctx)

	xlog.AddAttr(ctx, xlog.String("code", form.Code))

	if !xsession.EqualAndDelete(req.Context(), session, captionKey, form.Code) {
		// metric.LoginFailed()
		txt := anygo.Must1(xi18n.RC(i18nResource, req, "user/invalidCode"))
		webr.WriteJSON(w, 400, txt, nil)
		return
	}
	user, err := t.Board.UserCheckLogin(ctx, req, form.Name, form.Password)
	var authStr string
	if err == nil {
		authStr, err = t.authCookieValue(user)
	}
	if err != nil {
		t.clearLoginCookie(w)
		// metric.LoginFailed()
		xlog.AddAttr(ctx, xlog.ErrorAttr("error", err))
		txt := anygo.Must1(xi18n.RC(i18nResource, req, "user/loginFailed"))
		webr.WriteJSON(w, 2, txt, nil)
		return
	}

	authCookie := &http.Cookie{
		Name:     authCookieName,
		Value:    authStr,
		Path:     "/",
		Expires:  time.Now().AddDate(100, 0, 0),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, authCookie)
	_ = session.Save(ctx)

	txt := anygo.Must1(xi18n.RC(i18nResource, req, "user/loginSuc"))
	webr.WriteJSON(w, 0, txt, nil)
}

func (t *userHandler) clearLoginCookie(w http.ResponseWriter) {
	authCookie := &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, authCookie)
}

func (t *userHandler) logout(w http.ResponseWriter, r *http.Request) {
	se := xsession.FromContext(r.Context())
	_ = se.Clear(r.Context())
	t.clearLoginCookie(w)
	http.Redirect(w, r, t.Board.GetPathPrefix(), http.StatusFound)
}
