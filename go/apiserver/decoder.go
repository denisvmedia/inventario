package apiserver

import (
	"net/http"
	"strings"

	"github.com/go-chi/render"
)

// GetRequestContentType is a helper function that returns ContentType based on
// context or request headers.
func GetRequestContentType(r *http.Request) render.ContentType {
	if contentType, ok := r.Context().Value(render.ContentTypeCtxKey).(render.ContentType); ok {
		return contentType
	}
	return GetContentType(r.Header.Get("Content-Type"))
}

func GetContentType(s string) render.ContentType {
	s = strings.TrimSpace(strings.Split(s, ";")[0])
	switch s {
	case "text/plain":
		return render.ContentTypePlainText
	case "text/html", "application/xhtml+xml":
		return render.ContentTypeHTML
	case "application/json", "text/javascript", "application/vnd.api+json":
		return render.ContentTypeJSON
	case "text/xml", "application/xml":
		return render.ContentTypeXML
	case "application/x-www-form-urlencoded":
		return render.ContentTypeForm
	case "text/event-stream":
		return render.ContentTypeEventStream
	default:
		return render.ContentTypeUnknown
	}
}

func JSONAPIAwareDecoder(r *http.Request, v any) error {
	var err error

	switch GetRequestContentType(r) {
	case render.ContentTypeJSON:
		err = render.DecodeJSON(r.Body, v)
	case render.ContentTypeXML:
		err = render.DecodeXML(r.Body, v)
	case render.ContentTypeForm:
		err = render.DecodeForm(r.Body, v)
	default:
		err = ErrUnknownContentType
	}

	return err
}
