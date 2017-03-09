package optimus 

import (
	"github.com/go-openapi/loads"
	"errors"
	"github.com/fatih/structs"
	"github.com/julienschmidt/httprouter"
	"strings"
	"runtime" 
	"reflect"
)

type (

	Handler httprouter.Handle

	Transformer struct {
		Schemas map[string]map[string]core
		Server *httprouter.Router
		handlers []Handler 
	}

	core struct {
		schema loads.Document
		Harness map[string]map[string]settings
	}

	settings struct {
		Hidden bool 
		HandlerName string 
	}
)

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
			schema: *oaiSpec, 
			Harness : make(map[string]map[string]settings),
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

	server := httprouter.New()

	build(opt, server)
	
	return
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


func GetFunctionName(i interface{}) string {
    return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}