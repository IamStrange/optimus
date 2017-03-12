package optimus

import (
	"log"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

func (t *Transformer) validator(next httprouter.Handle) httprouter.Handle {
	fn := func(w http.ResponseWriter, request *http.Request, ps httprouter.Params) {
		pathPieces := strings.Split(request.URL.Path, "/")
		_, pathPieces = pathPieces[0], pathPieces[1:]
		base, pathPieces := pathPieces[0], pathPieces[1:]
		pathName := pathPieces[0]
		pathPieces = []string{}

		log.Println(pathName, base)

		next(w, request, ps)
		return
	}

	return fn
}
