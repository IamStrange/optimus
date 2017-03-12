package optimus

import (
	"errors"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	"github.com/fatih/structs"
	"github.com/go-openapi/loads"
	"github.com/julienschmidt/httprouter"
)

type (
	Handler httprouter.Handle

	Transformer struct {
		Schemas    map[string]map[string]core
		Server     *httprouter.Router
		handlers   []Handler
		middleware []func(httprouter.Handle) httprouter.Handle
	}

	core struct {
		schema  loads.Document
		Harness map[string]map[string]settings
	}

	settings struct {
		Hidden      bool
		HandlerName string
	}
)

func (s settings) SetHandlerName(name string) {
	s2 := &s
	s2.HandlerName = name
}

//New creates new optimus transformer for schemas
func New(schemas ...string) (opt *Transformer, err error) {

	var (
		oaiSpec *loads.Document
	)

	opt = new(Transformer)

	opt.Schemas = make(map[string]map[string]core)

	for _, schema := range schemas {
		oaiSpec, err = loads.JSONSpec(schema)
		if err != nil {
			err = errors.New("Something went wrong with loading schema. \n Look: " + schema + " and try again.")
			return
		}

		oaiSpec, err = oaiSpec.Expanded()
		if err != nil {
			err = errors.New("Something went wrong with loading schema. \n Look: " + schema + " and try again.")
			return
		}

		opt.Schemas[oaiSpec.Host()] = make(map[string]core)

		c := &core{
			schema:  *oaiSpec,
			Harness: make(map[string]map[string]settings),
		}

		swagger := oaiSpec.Spec()

		for path, PathItem := range swagger.Paths.Paths {
			//first up is path
			c.Harness[path] = make(map[string]settings)

			f := structs.Fields(PathItem.PathItemProps)

			for _, field := range f {
				if !field.IsZero() {
					opt.handlers = append(opt.handlers, fillerHandler(path))
					c.Harness[path][strings.ToLower(field.Name())] = settings{
						HandlerName: GetFunctionName(fillerHandler(path)),
					}

				}
			}
		}

		opt.Schemas[oaiSpec.Host()][oaiSpec.BasePath()] = *c
	}

	opt.Middleware(opt.responder, opt.validator)

	return
}

func (t *Transformer) AddHandler(fs ...Handler) *Transformer {
	//get function name
	for _, f := range fs {
		fnName := strings.ToLower(GetFunctionName(f))
		//split
		rawName := strings.Split(fnName, "/")
		//now get last el for base.func name should have model name it pertains to.
		//user . post
		// /user -> post
		name := strings.Replace(rawName[len(rawName)-1], "/", "", -1)
		name = strings.Replace(name, "-fm", "", -1)

		for host, base := range t.Schemas {
			for basepath, core := range base {
				for path, pathOpts := range core.Harness {
					for method, settings := range pathOpts {
						baseP := strings.Replace(basepath, "/", "", -1)
						p := strings.Replace(path, "/", "", -1)
						if name == baseP+".("+p+")."+method {
							settings.HandlerName = name
						}

						pathOpts[method] = settings
					}

					core.Harness[path] = pathOpts
				}

				base[basepath] = core
			}

			t.Schemas[host] = base
		}

		t.handlers = append(t.handlers, f)
	}

	return t
}

func (t *Transformer) Middleware(m ...func(httprouter.Handle) httprouter.Handle) *Transformer {
	for _, m := range m {
		t.middleware = append(t.middleware, m)
	}

	return t
}

func (t *Transformer) GetHandler(name string) (f Handler, err error) {
	for i, fn := range t.handlers {
		fn2Name := GetFunctionName(fn)
		if fn2Name == name {
			f = fn
			_, t.handlers = t.handlers[i], t.handlers[i:]
		}

	}
	if f == nil {
		err = errors.New("function can not be found")
	}
	return
}

func (t *Transformer) Wrap(fn httprouter.Handle) httprouter.Handle {
	t.middleware = reverseMiddleware(t.middleware)
	var f func(httprouter.Handle) httprouter.Handle
	middle :=
	for i, m := range t.middleware {
		if i == 0 {
			f = m(fn)
		} else {
			f = m(f)
		}

	}

	return f
}

func (t *Transformer) Run(port string) (err error) {
	server := httprouter.New()
	build(t, server)

	http.ListenAndServe(port, server)

	return
}

func GetFunctionName(i interface{}) string {
	f := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()

	fnName := strings.ToLower(f)
	//split
	rawName := strings.Split(fnName, "/")
	//now get last el for base.func name should have model name it pertains to.
	//user . post
	// /user-> post
	name := strings.Replace(rawName[len(rawName)-1], "/", "", -1)
	name = strings.Replace(name, "-fm", "", -1)
	return name
}
