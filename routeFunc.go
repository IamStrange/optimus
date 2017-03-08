package nofluff

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/gorilla/context"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func oaiHandler(resp http.ResponseWriter, request *http.Request) {

	var (
		host  = request.Host
		fpath = request.URL.Path
		base  string
	)
	//host fix
	piecesOfHost := strings.Split(host, ":")
	host = piecesOfHost[0]

	//base get
	piecesOfPath := strings.Split(fpath, "/")
	//use 1 since v2 is lead by a "/"
	base = "/" + piecesOfPath[1]
	schema := &registeredApis[host].Apis[base].Schema
	//get the swagger spec
	swag := schema.Spec()

	pathObjectSwagger := swag.Paths.Paths
	//now that we got base we need to make sure that base
	//is reverted to base string format. (minus the leading "/")
	//we added earlier.
	base = strings.Replace(base, "/", "", -1)
	for methodUpper, paths := range optimusHarness[host][base] {
		methodLower := strings.ToLower(methodUpper)
		method := UpcaseInitial(methodLower)

		pathObject, ok := paths.(map[string]interface{})

		if !ok {
			break
		}

		for path, settings := range pathObject {
			//harness doesnt have the "/"
			path = "/" + path
			operations, ok := pathObjectSwagger[path]

			if !ok {
				break
			}

			pathSettings := settings.(map[string]interface{})
			if val, ok := pathSettings["hidden"]; ok {
				hide, ok := val.(bool)
				if !ok {
					break
				}

				if hide == true {
					var empty bool
					operations, empty = cloakroutes(operations, method, path)
					if empty {
						delete(pathObjectSwagger, path)
					} else {
						pathObjectSwagger[path] = operations
					}
				}
			}
		}
	}

	swag.Paths.Paths = pathObjectSwagger
	//@TODO: move to a function
	b, err := json.MarshalIndent(swag, "", "  ")
	if err != nil {
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	resp.WriteHeader(http.StatusOK)
	resp.Write(b)
	return
}

func healthCheck(resp http.ResponseWriter, req *http.Request) {
	resp.WriteHeader(http.StatusOK)
	resp.Write([]byte("up"))
	return
}

//reDoc is a bake in, just a few script tags, but will be moving to a custom built 3 panel
func reDoc(w http.ResponseWriter, request *http.Request) {
	URL := request.URL
	path := URL.Path

	hostURL := strings.Replace(path, "docs", "_optimus/schema.json", 1)

	t, err := template.New("").ParseFiles(templatePath)

	if err != nil {
		http.Error(w, "Can not locate template for docs. Internal Server Error", http.StatusInternalServerError)
		return
	}

	err = t.ExecuteTemplate(w, "docs", hostURL)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	return
}

func wrap(h http.HandlerFunc, logging []alice.Constructor, m ...alice.Constructor) httprouter.Handle {

	b := base.Extend(alice.New(m...))

	if len(logging) > 0 {
		newBase := alice.New(logging...)
		loggingBase := newBase.Extend(b)

		return httpRouterWrap(loggingBase.ThenFunc(h))
	}

	return httpRouterWrap(b.ThenFunc(h))
}

func httpRouterWrap(h http.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		newWriter := httptest.NewRecorder()
		context.Set(r, "params", ps)
		h.ServeHTTP(newWriter, r)

		// we copy the original headers first
		for k, v := range newWriter.Header() {
			w.Header()[k] = v
		}

		w.WriteHeader(newWriter.Code)

		// But the Content-Length might have been set already,
		// we should modify it by adding the length
		// of our own data.
		// Ignoring the error is fine here:
		// if Content-Length is empty or otherwise invalid,
		// Atoi() will return zero,
		// which is just what we'd want in that case.
		clen, _ := strconv.Atoi(r.Header.Get("Content-Length"))
		w.Header().Set("Content-Length", strconv.Itoa(clen))
		// // then write out the original body
		w.Write(newWriter.Body.Bytes())
		context.Clear(r)
	}
}

func cloakroutes(operations spec.PathItem, method, path string) (spec.PathItem, bool) {

	//first check that the operations method is valid

	rOperations := reflect.ValueOf(operations)
	rMethod := rOperations.FieldByName(method)

	if !rMethod.IsValid() {
		return operations, false
	}

	switch method {
	case "Post":
		operations.Post = nil
	case "Get":
		operations.Get = nil
	case "Put":
		operations.Put = nil
	case "Delete":
		operations.Delete = nil
	case "Head":
		operations.Head = nil
	case "Patch":
		operations.Patch = nil
	case "Options":
		operations.Options = nil

	}

	//now get a nil cound... if 0 return nil
	//if more than 1 return original operations
	scannedOperationsCount := scan(operations)

	if scannedOperationsCount == 0 {
		return operations, true
	}
	return operations, false
}

func scan(operations spec.PathItem) int {
	total := 0

	if operations.Post != nil {
		total = total + 1
	}

	if operations.Get != nil {
		total = total + 1
	}

	if operations.Put != nil {
		total = total + 1
	}

	if operations.Delete != nil {
		total = total + 1
	}

	if operations.Head != nil {
		total = total + 1
	}

	if operations.Patch != nil {
		total = total + 1
	}

	if operations.Options != nil {
		total = total + 1
	}

	return total
}
