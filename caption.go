//  Copyright(C) 2026 github.com/hidu  All Rights Reserved.
//  Author: hidu <duv123+git@gmail.com>
//  Date: 2026-03-14

package nvwa

import (
	"net/http"

	"github.com/xanygo/anygo/store/xsession"
	"github.com/xanygo/anygo/xhttp"
	"github.com/xanygo/anygo/ximage/caption"
)

var codeForbidden = caption.NewAlphaNumber("Forbid")

func init() {
	codeForbidden.SetSize(100, 30)
}

const captionKey = "caption"

func codeHandler(w http.ResponseWriter, r *http.Request) {
	// if !metric.CaptionCanShow() {
	//	codeForbidden.ServeHTTP(w, r)
	//	return
	// }
	capt := caption.NewArithmetic()

	session := xsession.FromContext(r.Context())
	err := xsession.Set(r.Context(), session, captionKey, capt.Code())
	if err == nil {
		err = session.Save(r.Context())
	}
	if err != nil {
		xhttp.TextError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	capt.SetSize(100, 30)
	capt.ServeHTTP(w, r)
}
