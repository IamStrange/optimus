## OPTIMUS ECHO MIDDLEWARE
OAI/Swagger Intepreter for http://github.com/labstack/echo

#### Packages needed

#### How to use

before you download package. Please go have fun with http://editor.swagger.io


1. I had a primary focus of writing as little code as humany possible for an API at my company. 
In this focus, I developed Optimus. You will need to set an env variable for optimus to find all of your handlers.
(At this time) WE do handle subdomains, but do not handle multiple locations for handler code. 

```bash 
export OPTIMUS_HANDLER_PATH="/path/to/handler"
```

2. When you write your handlers you will need to make sure that Handler is at the end of the func Name. 
Also in OAI/Swagger doc. On a method for a route, you have operationId. I use this differently them. 
I used this as a "mapper for handlers." so if you have /v2/person as a GET and the operationId is "GetMyPerson"
the handler name should look like: GetMyPersonHandler

- side note: in the handlers folder the structure needs to look like this. 
- handlers/
    - hostname
        - middleware.go
        - basepath.go

```go 
func GetMyPersonHandler(c echo.Context) error
```

if you have middleware for that specific route. 
```go 
func GetMyPersonMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (e error) {
		}
    }
}
```


if you have global middleware for a basePath please put in the base folder as middleware.Go. 
The order the are written in (top down) is the order they will be registered with Echo. 

#### Coming soon
