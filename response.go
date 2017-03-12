package optimus

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (t *Transformer) responder(next httprouter.Handle) httprouter.Handle {
	fn := func(w http.ResponseWriter, request *http.Request, ps httprouter.Params) {
		log.Println("first")
		next(w, request, ps)

		return
	}

	return fn
}
