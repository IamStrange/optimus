package optimus

import (
	"github.com/julienschmidt/httprouter"
	"github.com/fatih/structs"
	"github.com/go-openapi/spec"
	"log"
	"net/http"
	"strings"
	"errors"
)

func build(opt *Transformer, server *httprouter.Router) {
	for _, baseSchema := range opt.Schemas {
		for base, schema := range baseSchema {
			doc := schema.schema.Spec()
			for path, ops := range doc.Paths.Paths {
				fullPath := base + path
				methods := schema.Harness[path]
				if err := operationSet(server, ops, fullPath, methods, opt); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func operationSet(server *httprouter.Router, ops spec.PathItem, fullPath string, methods map[string]settings, opt *Transformer) (err error) {
	fields := structs.Fields(ops.PathItemProps)
	for _, op := range fields {
		if !op.IsZero() {
			methodName := strings.ToLower(op.Name())
			if settings, ok := methods[methodName]; ok {
				switch methodName {
					case "post":
						if f, e := opt.GetHandler(settings.HandlerName); err == nil {
							if f == nil {
								err = errors.New("empty function")
								return 
							}

							server.POST(fullPath, httprouter.Handle(f))
							
						} else {
							err = e 
							return
						}
				} // end of switch			
			}			
		}
	}
	return
}

func fillerHandler(pathName string) Handler {
	return func (w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		w.Write([]byte(pathName + " up and running, please assign"))
		return
	}
}