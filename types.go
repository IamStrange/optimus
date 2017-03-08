package nofluff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/gorilla/context"
)

type (
	//hosts is the global wrapper around all apis
	//map[string] is api.talentiq.co (host) HOST/v2

	hostSwitcher map[string]http.Handler
	hosts        map[string]*host
	host         struct {
		//api basePath host/BASEPATH
		Apis map[string]*api
	}

	//each api itself
	api struct {
		ExpandedHandlers map[string]func(http.ResponseWriter, *http.Request)
		Schema           loads.Document
		securityFN       func(http.Handler) http.Handler
		Middleware       []func(http.Handler) http.Handler
		Override         override
		Logging          []func(http.Handler) http.Handler
	}

	settings struct {
		parameters     []spec.Parameter
		produces       []string
		consumes       []string
		schemes        []string
		securty        []*spec.SecurityScheme
		methodOverride bool
		method         map[string]settings
		responses      *spec.Responses
	}

	override struct {
		path map[string]settings
	}
)

// Implement the ServerHTTP method on our new type
func (hs hostSwitcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		base            string
		requestSettings = new(settings)
	)
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.

	host := strings.Split(r.Host, ":")[0]

	URL := r.URL
	//if is a docs route no middleware checks.

	if strings.Contains(URL.String(), ".js") || strings.Contains(URL.String(), "docs") || strings.Contains(URL.String(), "_optimus") {
		if handler := hs[host]; handler != nil {
			handler.ServeHTTP(w, r)
		}
		return
	}

	if _, ok := optimusHarness[host]; !ok {
		//error return
		return
	}

	hostHarness := optimusHarness[host]

	//now get the base path from url
	pathExploded := strings.Split(URL.Path, "/")

	if len(pathExploded) > 1 {
		base = "/" + pathExploded[1]
		_, pathExploded = pathExploded[0], pathExploded[2:]
	} else {
		base = "/" + pathExploded[0]
		_, pathExploded = pathExploded[0], pathExploded[1:]
	}

	baseHarness := make(map[string]interface{})
	var ok bool
	baseHarness, ok = hostHarness[strings.Replace(base, "/", "", 1)]

	if !ok {

		return
	}

	upMethod := strings.ToUpper(r.Method)
	if methodHarness, ok := baseHarness[upMethod]; !ok {

		return
	} else {
		if mHarness, ok := methodHarness.(map[string]interface{}); ok {
			requestPath := strings.Replace(r.URL.Path, base+"/", "", 1)
			if setting, ok := mHarness[requestPath]; ok {
				path := setting.(map[string]interface{})
				sec, ok := path["security"]

				if ok {
					settingSec, ok := sec.(bool)
					if ok {
						context.Set(r, "security_setting", settingSec)
					}
				}
			}
		}
	}

	if _, ok := registeredApis[host]; !ok {
		http.NotFound(w, r)
		return
	}

	api := registeredApis[host].Apis[base]

	context.Set(r, "optimus_core", api)
	sch := &api.Schema

	schemaSpec := sch.Spec()
	path := strings.Join(pathExploded, "/")
	if string(path[0]) != "/" {
		path = "/" + path
	}

	specPath, ok := schemaSpec.Paths.Paths[path]
	if !ok && path != "/docs" {
		http.NotFound(w, r)
		return
	}

	operation := getOperation(specPath, r.Method)
	if operation == nil {
		http.NotFound(w, r)
		return
	}

	context.Set(r, "optimus_operation", operation)

	//override here.
	//otherwise get shit from root.
	if len(api.Override.path) > 0 {
		if pathSettings, ok := api.Override.path[host+URL.Path]; ok {
			//path spec only allows param override
			pathParams := pathSettings.parameters
			if pathSettings.methodOverride {
				if methodSettings, ok := pathSettings.method[strings.ToLower(r.Method)]; ok {
					//ok now we know what method has an override and which dont.
					//now take settings and merge to requestSettings
					if len(methodSettings.parameters) > 0 {
						pathParams = methodSettings.parameters
						requestSettings.AddParameters(pathParams)
					}

					lockSettings(requestSettings, methodSettings)
				}
			}
		}
	}

	requestSettings.ConsumeSwagger(schemaSpec)
	context.Set(r, "optimus_settings", requestSettings)
	fmt.Println(host)
	if handler := hs[host]; handler != nil {
		handler.ServeHTTP(w, r)
	} else {
		// Handle host names for wich no handler is registered
		http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}

func (s *settings) AddResponse(resp *spec.Responses) {
	s.responses = resp
	return
}

//security is protected. Wrote adder
func (a *api) AddSecurity(f func(http.Handler) http.Handler) (ok bool) {
	ok = true
	if a.securityFN != nil {
		ok = false
	} else {
		a.securityFN = f
	}

	return
}

func (s *settings) AddMethod(method string, ms *settings) {

	if len(s.method) == 0 {
		s.method = make(map[string]settings)
	}

	if !s.methodOverride {
		s.methodOverride = true
	}

	blank := new(settings)

	if ms == blank {
		return
	}

	if _, ok := s.method[method]; !ok {
		s.method[method] = *ms
	}
}

func (s *settings) ConsumeSwagger(swag *spec.Swagger) {
	if len(s.consumes) == 0 {
		s.AddConsumes(swag.Consumes)
	}

	if len(s.produces) == 0 {
		s.AddProducers(swag.Produces)
	}

	if len(s.schemes) == 0 {
		s.AddSchemes(swag.Schemes)
	}

	if len(s.securty) == 0 {
		sec := []*spec.SecurityScheme{}
		for _, secDefinition := range swag.SecurityDefinitions {
			sec = append(sec, secDefinition)
		}
		s.AddSecurity(sec)
	}

	if len(s.parameters) == 0 {
		params := []spec.Parameter{}
		for _, param := range swag.Parameters {
			params = append(params, param)
		}
		s.AddParameters(params)
	}
}

func (s *settings) ValidateSchemes(ct string) bool {
	return searchSlice(s.schemes, ct)
}

func (s *settings) ValidateConsumes(ct string) bool {
	for _, c := range s.consumes {
		if strings.Contains(c, ct) || strings.Contains(ct, c) {
			return true
		}
	}

	return false
}

func (s *settings) ValidateProduces(ct string) bool {
	return searchSlice(s.produces, ct)
}

func (s *settings) ValidateParameters(request *http.Request, apiSpec *spec.Swagger) error {
	for _, val := range s.parameters {
		switch strings.ToLower(val.In) {
		case "body":
			contentType := request.Header.Get("Content-Type")

			if contentType == "application/json" {
				paramReadJSON := map[string]map[string]interface{}{}
				buf, _ := ioutil.ReadAll(request.Body)
				rdr1 := ioutil.NopCloser(bytes.NewBuffer(buf))
				rdr2 := ioutil.NopCloser(bytes.NewBuffer(buf))

				request.Body = rdr2
				readBody, e := ioutil.ReadAll(rdr1)

				if e != nil {
					return e
				}

				paramReadJSON["body"] = make(map[string]interface{})
				body := map[string]interface{}{}
				e = json.Unmarshal(readBody, &body)

				paramReadJSON["body"] = body

				if e != nil {
					return e
				}

				e = validate.AgainstSchema(val.Schema, body, strfmt.Default)
				if e != nil {
					return e
				}
			}

		case "form":
			formParams := request.Form
			if param, ok := formParams[val.Name]; ok {
				var v interface{}

				if val.Type == "string" {
					v, _ = param[0], param[1:]
				}

				//bug with passing []string
				result := validate.NewParamValidator(&val, strfmt.Default).Validate(v)
				if result != nil && result.AsError() != nil {
					return result.AsError()
				}
			} else {
				return errors.New("missing parameter")
			}

		case "path":
		case "query":
			rURL := request.URL
			value := rURL.Query().Get(val.Name)
			var queryValue interface{}

			switch {
			case val.Type == "integer":
				newValue, e := strconv.Atoi(value)
				if e != nil {
					return e
				}
				queryValue = newValue
			default:
				queryValue = value
			}

			result := validate.NewParamValidator(&val, strfmt.Default).Validate(queryValue)
			if result != nil && result.AsError() != nil {
				return result.AsError()
			}
		}
	}
	return nil
}

func (s *settings) ValidateSecurity(request *http.Request) bool {
	for _, secSettings := range s.securty {
		valid := true
		switch secSettings.In {
		case "header":
			if value := request.Header.Get(secSettings.Name); value == "" {
				valid = false
			}
		case "query":
			if value := request.URL.Query().Get(secSettings.Name); value == "" {
				valid = false
			}
		}
		if !valid {
			return valid
		}
	}

	return true
}

func (s *settings) AddSchemes(schemes []string) {
	s.schemes = schemes
}

func (s *settings) AddConsumes(c []string) {
	s.consumes = c
}

func (s *settings) AddProducers(p []string) {
	s.produces = p
}

func (s *settings) AddSecurity(security []*spec.SecurityScheme) {
	s.securty = security
}

func (s *settings) AddParameters(params []spec.Parameter) {
	s.parameters = params
}

func (o *override) AddPath(path string, sts *settings) {
	if len(o.path) == 0 {
		o.path = make(map[string]settings)
	}
	if _, ok := o.path[path]; !ok {
		o.path[path] = *sts
	}
}

// //master_hosts is the recipe for echo multi domains

//harness is custom to optimus
type harness map[string]map[string]map[string]interface{}

//Func Store is used to registering a new api / spec with
//optimus.

func (h hosts) ExpandAllHandlers() hosts {
	for hostLiteral, hostSettings := range h {
		for base := range hostSettings.Apis {
			handlerMapping := hostLiteral + base
			for _, handlerFunc := range registeredHandlers {

				handlerNameRaw := getFuncName(handlerFunc)
				if strings.Contains(handlerNameRaw, handlerMapping) {
					handlerNameExploded := strings.Split(handlerNameRaw, ".")
					handlerName := handlerNameExploded[len(handlerNameExploded)-1]
					fmt.Println(handlerFunc)
					h[hostLiteral].Apis[base].ExpandedHandlers[handlerName] = handlerFunc
				}
			}
		}
	}

	return h
}

func (h hosts) Store(filePath string) (E error) {
	var (
		e   error
		API *api
	)

	API = new(api)
	API.ExpandedHandlers = make(map[string]func(http.ResponseWriter, *http.Request))
	oaiSpec, e := loads.JSONSpec(filePath)
	if e != nil {
		E = e
		return
	}

	oaiSpec, e = oaiSpec.Expanded()
	if e != nil {
		E = e
		return
	}

	var hostName = oaiSpec.Host()
	var base = oaiSpec.BasePath()

	API.Schema = *oaiSpec

	if _, ok := h[hostName]; !ok {
		newHost := new(host)
		newHost.Apis = make(map[string]*api)
		newHost.Apis[base] = API
		h[hostName] = newHost
	} else {
		hostRecord := h[hostName]
		if _, ok := hostRecord.Apis[base]; !ok {
			hostRecord.Apis[base] = API
		} else {
			E = errors.New("Api with host of: " + hostName + " and base of: " + base + " is a dup.")
		}
	}

	return
}
