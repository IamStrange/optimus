package nofluff

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
	"github.com/rs/cors"
)

var (
	base     = alice.New(ravage, perceptor, validateParams)
	switcher = make(hostSwitcher)
)

func build(port string) (cybertron *httprouter.Router) {
	registeredApis = registeredApis.ExpandAllHandlers()
	for hostLiteral, apiHost := range registeredApis {
		HostRouter := httprouter.New()
		HostRouter.NotFound = http.FileServer(http.Dir(os.Getenv("GOPATH") + "/bin/assets"))
		for base_path, API := range apiHost.Apis {
			var (
				router_path_base = base_path
				harnessSettings  = make(map[string]interface{})
				ok               bool
			)

			if _, ok = optimusHarness[hostLiteral]; !ok {
				return
			}

			if harnessSettings, ok = optimusHarness[hostLiteral][base_path]; !ok {
				if harnessSettings, ok = optimusHarness[hostLiteral][strings.Replace(base_path, "/", "", 1)]; !ok {
					return
				}
			}

			HostRouter.GET("/h", wrap(healthCheck, []alice.Constructor{}))

			//add our optimus sweetness
			transformationCog(HostRouter, router_path_base)

			//securit function

			if securityFNName, ok := harnessSettings["security"]; ok {
				securityFN := findSecurityFuncs(securityFNName)
				if securityFN == nil {
					return
				}
				//securiy added...
				//now lets loop over get, post, anything but security and middleware keeywords
				API.AddSecurity(securityFN)
				delete(harnessSettings, "security")

			}

			// //now get the base_path settings from harness.
			if middlewareList, ok := harnessSettings["middleware"]; ok {
				//loop over middlewareList
				list, ok := middlewareList.([]interface{})
				if !ok {
					return
				}

				//mFn middleware Function name
				mFn := findMiddlewareFuncs(list...)

				// if len == 0 something failed
				if len(mFn) == 0 {
					return
				}

				API.Middleware = append(API.Middleware, mFn...)

				delete(harnessSettings, "middleware")
			}

			// //now get the base_path settings from harness.
			if middlewareList, ok := harnessSettings["LOGGING"]; ok {
				//loop over middlewareList
				list, ok := middlewareList.([]interface{})
				if !ok {
					return
				}
				//mFn middleware Function name
				mFn := findLoggingFuncs(list...)
				// if len == 0 something failed
				if len(mFn) == 0 {
					return
				}

				API.Logging = append(API.Logging, mFn...)

				delete(harnessSettings, "LOGGING")
			}

			schema := &API.Schema
			swag := schema.Spec()
			if swag.Paths == nil {
				return
			}

			//loop over the paths

			for path, operations := range swag.Paths.Paths {

				for method, settings := range harnessSettings {
					paths := settings.(map[string]interface{})

					pathName := strings.Replace(path, "/", "", 1)

					pathConfig, ok := paths[pathName]

					if !ok {
						continue
					}

					pathConfigs, ok := pathConfig.(map[string]interface{})

					if !ok {
						continue
					}

					//the ENV pertains to which routes should be mapped based on
					//harness host_base_ENV=VALUE
					/*
					   www.domain.com/v1
					   $ export www_domain_com_v1_ENV=API
					*/

					envConst, ok := pathConfigs["ENV"]

					if !ok {
						return
					}

					envCheck := "_"
					envCheck = envCheck + strings.Replace(hostLiteral, ".", "_", -1)
					envCheck = strings.Replace(envCheck, "-", "_", -1)
					envCheck = envCheck + "_" + strings.Replace(base_path, "/", "", 1)
					envCheck += "_ENV"
					systemENV := os.Getenv(envCheck)

					if systemENV != envConst {
						continue
					}

					userHandler, ok := pathConfigs["handler"]

					if !ok {
						continue
					}

					userHandlerName, ok := userHandler.(string)

					if !ok {
						continue
					}

					f, ok := API.ExpandedHandlers[userHandlerName]

					if !ok {

						continue
					}

					routePath := router_path_base + path
					fmt.Println(path)
					contructedAlice := contructAlice(API.Middleware)
					contructedLoggingAlice := contructAlice(API.Logging)
					connector(HostRouter, operations, f, method, routePath, contructedLoggingAlice, contructedAlice...)
				}
			}

			//reassign
			apiHost.Apis[router_path_base] = API
		}

		switcher[hostLiteral] = HostRouter

		registeredApis[hostLiteral] = apiHost
	}
	prioritize()
	//delete all registerdHandlers from security and middleware
	registeredSecurity = []func(http.Handler) http.Handler{}
	registeredMiddleware = []func(http.Handler) http.Handler{}
	registeredLogging = []func(http.Handler) http.Handler{}

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"*"},
	})

	handler := c.Handler(switcher)
	log.Fatal(http.ListenAndServe(port, handler))

	return
}

func contructAlice(m []func(http.Handler) http.Handler) []alice.Constructor {
	whiteRabbit := []alice.Constructor{}

	for _, mid := range m {
		whiteRabbit = append(whiteRabbit, mid)
	}

	return whiteRabbit
}

//transformationCog is used for building custom routes into the api schema
//that allows it to have full functionality of optimus
func transformationCog(router *httprouter.Router, base_path string) {
	//route for redoc bake in
	router.GET(base_path+"/docs", wrap(reDoc, []alice.Constructor{}))
	router.GET(base_path+"/_optimus/schema.json", wrap(oaiHandler, []alice.Constructor{}))
}

// //connector is like a "transformers connecting pieces"
// //their shoulders and such... It connects the group with all the methods needed
func connector(router *httprouter.Router, operations spec.PathItem, f func(http.ResponseWriter, *http.Request), method, path string, logging []alice.Constructor, middleware ...alice.Constructor) {

	fieldFormat := strings.Title(strings.ToLower(method))
	methodReflect := reflect.ValueOf(operations).FieldByName(fieldFormat)
	if !methodReflect.IsValid() {
		return
	}

	switch fieldFormat {
	case "Post":
		router.POST(path, wrap(f, logging, middleware...))
	case "Get":
		router.GET(path, wrap(f, logging, middleware...))
	case "Put":
		router.PUT(path, wrap(f, logging, middleware...))
	case "Delete":
		router.DELETE(path, wrap(f, logging, middleware...))
	case "Head":
		router.HEAD(path, wrap(f, logging, middleware...))
	case "Patch":
		router.PATCH(path, wrap(f, logging, middleware...))
	case "Options":
		router.OPTIONS(path, wrap(f, logging, middleware...))

	}
}

// //returns function name
// //probably needs proper error catching
func getFuncName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

//middlewares are generally used in bunches
//hence lists
func findMiddlewareFuncs(fs ...interface{}) (allMiddleware []func(http.Handler) http.Handler) {
	for _, f := range fs {
		funcName := f.(string)
		var fn func(http.Handler) http.Handler
		for _, f2 := range registeredMiddleware {
			fN := getFuncName(f2)
			fNSplit := strings.Split(fN, "/")
			literalFuncName := strings.Split(fNSplit[len(fNSplit)-1], ".")
			if literalFuncName[len(literalFuncName)-1] == funcName {
				fn = f2
				break
			}
		}

		allMiddleware = append(allMiddleware, fn)
	}
	return
}

func findLoggingFuncs(fs ...interface{}) (allLogging []func(http.Handler) http.Handler) {
	for _, f := range fs {
		funcName := f.(string)
		var fn func(http.Handler) http.Handler
		for _, f2 := range registeredLogging {
			fN := getFuncName(f2)
			fNSplit := strings.Split(fN, "/")
			literalFuncName := strings.Split(fNSplit[len(fNSplit)-1], ".")
			if literalFuncName[len(literalFuncName)-1] == funcName {
				fn = f2
				break
			}
		}

		allLogging = append(allLogging, fn)
	}
	return
}

//finds the current security function based off of optimus harness.json
func findSecurityFuncs(f interface{}) func(http.Handler) http.Handler {
	funcName, err := f.(string)

	if !err {
		return nil
	}

	var fn func(http.Handler) http.Handler
	for _, f2 := range registeredSecurity {
		fN := getFuncName(f2)
		fNSplit := strings.Split(fN, "/")
		literalFuncName := strings.Split(fNSplit[len(fNSplit)-1], ".")
		if literalFuncName[1] == funcName {
			fn = f2
			break
		}
	}
	return fn
}

// //prioritize the shit we need to know for quicker grabbing

func prioritize() {
	for host, api := range registeredApis {
		for base, api := range api.Apis {
			newOverride := new(override)

			schema := &api.Schema
			swagger := schema.Spec()

			for path, operations := range swagger.Paths.Paths {

				sets := new(settings)

				builtURL := host + base + path
				pathParams := operations.Parameters
				if len(pathParams) != 0 {
					sets.AddParameters(pathParams)
				}

				methodPrioritize(operations.Post, sets, "post")
				methodPrioritize(operations.Get, sets, "get")
				methodPrioritize(operations.Put, sets, "put")
				methodPrioritize(operations.Delete, sets, "delete")
				methodPrioritize(operations.Head, sets, "head")
				methodPrioritize(operations.Patch, sets, "patch")

				newOverride.AddPath(builtURL, sets)
			}

			api.Override = *newOverride
			registeredApis[host].Apis[base] = api
		}
	}
}

func methodPrioritize(operation *spec.Operation, sets *settings, ms string) {

	if operation == nil {
		return
	}

	methodSettings := new(settings)

	consumes := operation.Consumes
	produces := operation.Produces
	parameters := operation.Parameters
	schemes := operation.Schemes

	methodSettings.AddResponse(operation.Responses)

	if len(consumes) > 0 {
		methodSettings.AddConsumes(consumes)
	}

	if len(produces) > 0 {
		methodSettings.AddProducers(produces)
	}

	if len(parameters) > 0 {
		methodSettings.AddParameters(parameters)
	}

	if len(schemes) > 0 {
		methodSettings.AddSchemes(schemes)
	}

	sets.AddMethod(ms, methodSettings)

}
