package optimus

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/fatih/structs"
	"github.com/go-openapi/spec"
	"github.com/julienschmidt/httprouter"
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

						fn := opt.Wrap(httprouter.Handle(f))

						server.POST(fullPath, fn)
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

func reverseMiddleware(array []func(httprouter.Handle) httprouter.Handle) []func(httprouter.Handle) httprouter.Handle {

	for i, j := 0, len(array)-1; i < j; i, j = i+1, j-1 {
		array[i], array[j] = array[j], array[i]
	}
	log.Println(len(array))
	return array
}

func fillerHandler(pathName string) Handler {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		w.Write([]byte(pathName + " up and running, please assign"))
		return
	}
}
