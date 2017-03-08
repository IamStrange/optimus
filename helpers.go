package nofluff

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/asaskevich/govalidator"
)

func fileORurl(filePath string) (ok bool) {
	ok = (!govalidator.IsURL(filePath) && !fileExists(filePath))
	ok = !ok
	return
}

// exists returns whether the given file or directory exists or not
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func loadharness(path string) harness {

	var (
		err         error
		byteHarness []byte
	)

	h := make(harness)
	if !fileORurl(path) {
		return h
	}

	switch {
	case govalidator.IsURL(path):
		response, e := http.Get(path)
		if e != nil {
			return h
		}

		byteHarness, err = ioutil.ReadAll(response.Body)
	case fileExists(path):
		byteHarness, err = ioutil.ReadFile(path)
	}

	if err != nil {
		return h
	}

	err = json.Unmarshal(byteHarness, &h)
	if err != nil {
		return h
	}

	//uppercase methods
	for host, baseSettings := range h {
		for base, methodsSettings := range baseSettings {
			for method, settings := range methodsSettings {
				if method != "security" && method != "middleware" {
					delete(h[host][base], method)
					method = strings.ToUpper(method)
				}
				h[host][base][method] = settings
			}
		}
	}

	return h
}

func searchSlice(slice []string, find string) bool {
	if find == "" {
		return false
	}

	found := false
	for _, value := range slice {
		if value == find {
			found = true
			break
		}
	}
	return found
}

//UpcaseInitial is used in placement of strings.Title
//doesn't work for some reason
func UpcaseInitial(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
