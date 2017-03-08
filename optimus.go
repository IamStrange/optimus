package nofluff

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

//RFC339
//2016-11-17T17:43:21-08:00

var (
	//can set the public config
	templatePath = os.Getenv("OPTIMUS_PUBLIC")

	registeredHandlers   []func(http.ResponseWriter, *http.Request)
	registeredMiddleware []func(http.Handler) http.Handler
	registeredSecurity   []func(http.Handler) http.Handler
	registeredLogging    []func(http.Handler) http.Handler
	//registration piece
	registeredApis = make(hosts)
	optimusHarness harness
)

//Transform is what is entry point for the package. It takes the echo server
//and file_paths that lead to handlers and maps the server to schemas, to handlers and returns a running echo server client.
func Transform(port string, filePaths ...string) (err error) {
	//verify the paths passed are valid.
	if templatePath == "" {
		//if not attempt package route

		templatePath = os.Getenv("GOPATH") + "/bin/public/views/index.html"
	}
	fmt.Println(templatePath)
	fmt.Println("optimus waking up", time.Now().Format(time.RFC3339))

	for _, filePath := range filePaths {
		if !fileORurl(filePath) {
			err = errors.New("schema location: " + filePath + " does not exist.")
			return
		}

		registeredApis.Store(filePath)
	}

	path := os.Getenv("OPTIMUS_CONFIG")
	if !fileORurl(path) {
		err = errors.New("harness location: " + path + " does not exist.")
		return
	}

	optimusHarness = loadharness(path)

	if port == "" {
		port = ":5050"
	} else {
		port = ":" + port
	}

	_ = build(port)
	return
}

//RegisterHandler registers the echo handler with optimus internal services
func RegisterHandler(f ...func(http.ResponseWriter, *http.Request)) {
	registeredHandlers = append(registeredHandlers, f...)

}

//RegisterMiddleware registers middleware
func RegisterMiddleware(f ...func(http.Handler) http.Handler) {
	registeredMiddleware = append(registeredMiddleware, f...)
}

//RegisterMiddleware registers middleware
func RegisterLogging(f ...func(http.Handler) http.Handler) {
	registeredLogging = append(registeredLogging, f...)
}

//RegisterSecurity registers authentication middleware seperately because of how optimus runs
func RegisterSecurity(f ...func(http.Handler) http.Handler) {
	registeredSecurity = append(registeredSecurity, f...)
}
