package nofluff

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/gorilla/context"
)

func perceptor(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		URL := request.URL

		URL.Scheme = "http"

		switch {

		case strings.Contains(URL.Path, ".js"):
			fallthrough
		case strings.Contains(URL.Path, "/docs"):
			fallthrough
		case strings.Contains(URL.Path, "/_optimus"):
			next.ServeHTTP(w, request)
			return
		}

		//now we have all the shit we need.
		contentType := request.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "text/plain"
		}

		requestSettings := context.Get(request, "optimus_settings")
		settings, ok := requestSettings.(*settings)

		optimusCore := context.Get(request, "optimus_core")
		api, ok := optimusCore.(*api)

		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if len(settings.consumes) > 0 && !settings.ValidateConsumes(contentType) {
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			return
		}

		if URL.Scheme != "" && !settings.ValidateSchemes(URL.Scheme) {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}

		if !settings.ValidateSecurity(request) {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		security := context.Get(request, "security_setting")
		runSecurity := true

		if security != nil {
			if rs, ok := security.(bool); ok {
				runSecurity = rs
			}
		}

		if api.securityFN != nil && runSecurity {
			api.securityFN(next).ServeHTTP(w, request)
		} else {
			next.ServeHTTP(w, request)
		}

		return
	}

	return http.HandlerFunc(fn)
}

func validateParams(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		URL := request.URL
		switch {
		case strings.Contains(URL.Path, "/docs"):
			fallthrough
		case strings.Contains(URL.Path, "/_optimus"):
			fallthrough
		case strings.Contains(URL.Path, "/assets"):
			next.ServeHTTP(w, request)
			return
		}
		optimusCore := context.Get(request, "optimus_core")

		api, ok := optimusCore.(*api)

		if !ok {
			http.Error(w, "major erros in core of optimus", http.StatusInternalServerError)
			return
		}

		sch := &api.Schema
		schemaSpec := sch.Spec()

		requestSettings := context.Get(request, "optimus_settings")
		settings, ok := requestSettings.(*settings)

		if err := settings.ValidateParameters(request, schemaSpec); err != nil {
			context.Set(request, "validation:Errors", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		next.ServeHTTP(w, request)
		return
	}
	return http.HandlerFunc(fn)
}

func getOperation(sp spec.PathItem, method string) (op *spec.Operation) {
	m := UpcaseInitial(strings.ToLower(method))
	reflectPath := reflect.ValueOf(sp)
	field := reflectPath.FieldByName(m)

	if field.IsValid() {
		operation := field.Interface()
		op = operation.(*spec.Operation)
	}

	return
}

func lockSettings(set *settings, merging settings) {

	if len(set.parameters) == 0 && len(merging.parameters) > 0 {
		set.AddParameters(merging.parameters)
	}

	if len(set.consumes) == 0 && len(merging.consumes) > 0 {
		set.AddConsumes(merging.consumes)
	}

	if len(set.produces) == 0 && len(merging.produces) > 0 {
		set.AddProducers(merging.produces)
	}

	if len(set.schemes) == 0 && len(merging.schemes) > 0 {
		set.AddSchemes(merging.schemes)
	}

	if len(set.securty) == 0 && len(merging.securty) > 0 {
		set.AddSecurity(merging.securty)
	}

	set.AddResponse(merging.responses)
}
