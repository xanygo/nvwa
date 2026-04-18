//  Copyright(C) 2026 github.com/hidu  All Rights Reserved.
//  Author: hidu <duv123+git@gmail.com>
//  Date: 2026-03-14

package nvwa

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"net/http"
	"path"
	"sync"

	"github.com/xanygo/anygo/ds/xmap"
	"github.com/xanygo/anygo/ds/xsync"
	"github.com/xanygo/anygo/xcodec"
	"github.com/xanygo/anygo/xcodec/xbase"
	"github.com/xanygo/anygo/xhtml"
	"github.com/xanygo/anygo/xhttp"
	"github.com/xanygo/anygo/xhttp/xhandler"
	"github.com/xanygo/anygo/xi18n"
	"github.com/xanygo/anygo/xio/xfs"
	"github.com/xanygo/anygo/xlog"
	"github.com/xanygo/webr"
)

type Dashboard struct {
	PathPrefix    string        // uri 地址前缀，若为空，则默认为 /admin/
	AssetPrefix   string        // 静态资源地址前缀，若为空，则默认为 /asset/
	RegisterAsset string        // 在 PathPrefix 下注册 asset 的目录名，若有值则会注册
	Bundle        *xi18n.Bundle // 国际化资源，可选

	FuncMap template.FuncMap // 可选，注册到模版中的自定义方法

	SecretKey string // 加密秘钥，必填
	cipher    xcodec.Cipher

	TemplateFS fs.FS                                                          // 模版文件，必填
	tpl        *xmap.Tags[xi18n.Language, *template.Template, xi18n.Language] // 由 TemplateFS 编译得到的模版

	Router *xhttp.Router // 路由，可选

	UserFinder     func(ctx context.Context, name string) (*User, error)                                // 操作用户，必填
	UserCheckLogin func(ctx context.Context, req *http.Request, name string, psw string) (*User, error) // 用户登录，必填

	once sync.Once
}

func (db *Dashboard) InitOnce() {
	db.once.Do(db.doInit)
}

func (db *Dashboard) doInit() {
	funcMap := db.funcMap()
	maps.Copy(funcMap, xhtml.FuncMap)
	db.tpl = xi18n.BuildTemplate(xi18n.LangZh, db.Bundle, func(t *template.Template) *template.Template {
		t = t.Funcs(funcMap).Funcs(db.FuncMap)
		return template.Must(xhtml.WalkParseFS(t, db.TemplateFS, ".", "*.html"))
	})

	if db.AssetPrefix != "" {
		db.AssetPrefix = path.Clean(db.AssetPrefix) + "/"
	}

	db.cipher = &xcodec.AesOFB{
		Key: db.SecretKey,
	}

	uh := &userHandler{
		Board: db,
	}
	router := db.HTTPRouter()

	if db.RegisterAsset != "" {
		ah := xhttp.FSHandlers{
			&xhttp.FS{
				FS: xfs.OverlayFS{
					webr.Bootstrap(),
					webr.Icons(),
					webr.JQuery(),
					webr.Clipboard(),
					webr.Sortable(),
					webr.UI(),
				},
			},
		}
		mh := &xhttp.FSMerge{
			Minify: map[string]func(b []byte) ([]byte, error){},
			FS:     ah,
		}
		at := &xhandler.AntiTheft{}
		router.Get(db.RegisterAsset+"/*", http.StripPrefix(db.GetAssetPrefix(), mh), at.Next)
	}

	router.GetFunc("/code", codeHandler)
	router.GetFunc("/login", uh.loginIndex)
	router.PostFunc("/login", uh.loginCheck)
	router.GetFunc("/logout", uh.logout)
	router.Use(uh.checkLogin)
}

func (db *Dashboard) GetPathPrefix() string {
	if db.PathPrefix != "" {
		return db.PathPrefix
	}
	return "/admin/"
}

func (db *Dashboard) GetAssetPrefix() string {
	if db.AssetPrefix != "" {
		return db.AssetPrefix
	}
	return "/asset/"
}

func (db *Dashboard) HTTPRouter() *xhttp.Router {
	if db.Router == nil {
		db.Router = &xhttp.Router{}
	}
	return db.Router
}

var _ http.Handler = (*Dashboard)(nil)

func (db *Dashboard) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	db.InitOnce()
	db.HTTPRouter().ServeHTTP(w, req)
}

// Render 渲染外部注册资源的模版文件
func (db *Dashboard) Render(ctx context.Context, w http.ResponseWriter, req *http.Request, fileName string, data map[string]any) {
	if data == nil {
		data = make(map[string]any)
	}
	if _, ok := data["deps"]; !ok {
		data["deps"] = &xhtml.Deps{}
	}
	data["TR"] = xhtml.NewTPLRequest(req)
	data["nvwa_user"] = UserFormContext(req.Context())
	data["nvwa_asset"] = db.fnNvwaAssetPath

	languages := xi18n.LanguagesFromContext(ctx)
	err := db.tpl.Any(xi18n.LangZh, languages...).ExecuteTemplate(w, fileName, data)
	if err != nil {
		_, _ = w.Write([]byte("Render failed:" + template.HTMLEscapeString(err.Error())))
		xlog.Warn(ctx, "Render failed", xlog.String("fileName", fileName), xlog.ErrorAttr("error", err))
	}
}

// RenderWithLayout 渲染外部注册资源的模版文件,然后将内容渲染到模版文件
func (db *Dashboard) RenderWithLayout(ctx context.Context, w http.ResponseWriter, req *http.Request, fileName string, data map[string]any) {
	if data == nil {
		data = make(map[string]any)
	}
	deps := &xhtml.Deps{}
	data["deps"] = deps

	if xhttp.IsAjax(req) {
		db.Render(ctx, w, req, fileName, data)
		return
	}
	data["TR"] = xhtml.NewTPLRequest(req)
	data["nvwa_asset"] = db.fnNvwaAssetPath
	data["nvwa_user"] = UserFormContext(req.Context())

	bf := xsync.GetBytesBuffer()
	languages := xi18n.LanguagesFromContext(ctx)
	tpl := db.tpl.Any(xi18n.LangZh, languages...)
	err := tpl.ExecuteTemplate(bf, fileName, data)
	if err == nil {
		data["Body"] = bf.String()
		xsync.PutBytesBuffer(bf)
		err = tpl.ExecuteTemplate(w, "layout.html", data)
	}

	if err != nil {
		_, _ = w.Write([]byte("RenderWithLayout failed:" + template.HTMLEscapeString(err.Error())))
		xlog.Warn(ctx, "RenderWithLayout failed", xlog.String("fileName", fileName), xlog.ErrorAttr("error", err))
	}
}

func (db *Dashboard) fnNvwaAssetPath(name string) template.URL {
	return template.URL(xhttp.PathJoin(db.GetAssetPrefix(), name))
}

func (db *Dashboard) funcMap() template.FuncMap {
	return template.FuncMap{
		"rel_link": func(str string, args ...any) template.URL {
			v := fmt.Sprintf(str, args...)
			return template.URL(v)
		},
		"abs_link": func(str string, args ...any) template.URL {
			v := fmt.Sprintf(str, args...)
			return template.URL(xhttp.PathJoin(db.GetPathPrefix(), v))
		},
	}
}

func (db *Dashboard) AbsLink(uri string) string {
	return xhttp.PathJoin(db.GetPathPrefix(), uri)
}

func (db *Dashboard) encrypt(text string) (string, error) {
	bf, err := db.cipher.Encrypt([]byte(text))
	if err != nil {
		return "", err
	}
	return xbase.Base58.EncodeToString(bf), nil
}

func (db *Dashboard) decrypt(text string) (string, error) {
	bf, err := xbase.Base58.DecodeString(text)
	if err != nil {
		return "", err
	}
	val, err := db.cipher.Decrypt(bf)
	if err != nil {
		return "", err
	}
	return string(val), nil
}
