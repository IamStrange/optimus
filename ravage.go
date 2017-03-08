package nofluff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	uuid "github.com/satori/go.uuid"

	"bytes"

	"github.com/gorilla/context"
)

func ravage(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(w, request)
		URL := request.URL
		switch {
		case strings.Contains(URL.Path, "/docs"):
			fallthrough
		case strings.Contains(URL.Path, "/_optimus"):
			fallthrough
		case strings.Contains(URL.Path, ".js"):
			return
		}

		var (
			statusCode int
		)

		response := w.(*httptest.ResponseRecorder)
		statusCode = response.Code

		requestSettings := context.Get(request, "optimus_settings")
		settings, ok := requestSettings.(*settings)

		if !ok {
			return
		}

		if settings == nil && settings.responses == nil {
			w.Header().Set("X-OPTIMUS-NO-RESPONSE", "No responses defined")
			return
		}

		responses := settings.responses

		if responses == nil {
			return
		}

		responseSchema, ok := responses.StatusCodeResponses[statusCode]

		if !ok && responses.Default != nil {
			responseSchema = *responses.Default
		}

		// for headerKey, h := range responseSchema.Headers {
		// 	if headerValue := response.HeaderMap.Get(headerKey); headerValue != "" {
		// 		var hValue interface{}

		// 		switch h.Type {
		// 		case "integer":
		// 			toInt, e := strconv.Atoi(headerValue)
		// 			if e != nil {

		// 			}
		// 			hValue = toInt
		// 		default:
		// 			hValue = headerValue
		// 		}

		// 		headerValidator := validate.NewHeaderValidator(headerKey, &h, strfmt.Default)
		// 		validationErrors := headerValidator.Validate(hValue)
		// 		if validationErrors != nil {
		// 			response.Header().Set("X-OPTIMUS-HEADERS", "NOT SET ACCORDING TO SCHEMA.")
		// 		}
		// 	}
		// }

		var body interface{}

		contentType := request.Header.Get("content-type")
		validErr := context.Get(request, "validation:Errors")
		validation := []string{}
		if validErr != nil {
			masterError, ok := validErr.(error)
			if !ok {
				return
			}

			pieces := strings.Split(masterError.Error(), "\n")

			for _, p := range pieces {
				if strings.Contains(p, "validation") {
					continue
				}
				validation = append(validation, strings.Trim(p, "\n\t "))
			}
		}

		//if not a 200 formulate response
		if responseSchema.Schema != nil && (statusCode > 299 || statusCode < 200) {
			newbody := map[string]interface{}{}

			for key, settings := range responseSchema.Schema.SchemaProps.Properties {
				if key == "errors" && settings.Type.Contains("object") {
					errorBody := map[string]interface{}{}

					if _, ok := settings.Properties["message"]; ok {
						errorBody["message"] = http.StatusText(statusCode)
					}

					if len(validation) > 0 {
						if _, ok := settings.Properties["validation"]; ok && len(validation) > 0 {
							errorBody["validation"] = validation
						}
					}

					newbody["errors"] = errorBody
				}

				if key == "status" {
					newbody[key] = response.Code
				}

				if settings.Format == "uuid" {
					newbody[key] = uuid.NewV4()
				}
			}

			if len(newbody) > 0 {
				response.Flush()
				response.Code = statusCode
				newBodyBytes, _ := json.MarshalIndent(newbody, "", " ")
				response.Header().Set("Content-Type", "application/json")
				response.Body = bytes.NewBuffer(newBodyBytes)

				return
			}
		}
		_ = body
		_ = contentType
		// switch {
		// case contentType == "application/json" || searchSlice(settings.produces, "application/json"):
		// 	err := json.Unmarshal(response.Body.Bytes(), &body)
		// 	if err != nil {

		// 	}

		// }

		// //headers validated

		// if responseSchema.Schema != nil {
		// 	err := validate.AgainstSchema(responseSchema.Schema, body, strfmt.Default)
		// 	//@TODO: write out head to let users know a code change happened but not a schema change
		// 	fmt.Println(err)
		// }
	}

	return http.HandlerFunc(fn)
}
