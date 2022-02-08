---
description: The web API interface allows access to functionality of the node software via exposed HTTP endpoints.
image: /img/logo/goshimmer_light.png
keywords:
- web API
- POST
- GET
- node software
- http endpoint
- port 
- handler
---
# WebAPI - clientLib

The web API interface allows access to functionality of the node software via exposed HTTP endpoints.

## How to Use the API 

The default port to access the web API is set to `8080:8080/tcp` in `docker-compose.yml`, where the first port number is the internal port number within the node software, and the second for the access from an http port. An example where these two would be set to different values, or the external port is not utilized, can be found in the docker-network tool (see also the `docker-compose.yml` file in the docker-network tool folder).

The server instance of the web API is contacted via `webapi.Server()`. Next we need to register a route with a matching handler.

```go
webapi.Server().ROUTE(path string, h HandlerFunc)
```
where `ROUTE` will be replaced later in this documentation by `GET` or `POST`. The `HandlerFunc` defines a function to serve HTTP requests that gives access to the Context

```go
func HandlerFunc(c Context) error
```
We can then use the Context to send a JSON response to the node: 
```go
JSON(statuscode int, i interface{}) error
```
An implementation example is shown later for the POST method.

## GET and POST 

Two methods are currently used. First, with `GET` we register a new GET route for a handler function. The handler is accessed via the address `path`. The handler for a GET method can set the node to perform certain actions.
```go
webapi.Server().GET("path", HandlerFunc)
```
A command can be sent to the node software to the API, e.g. via command prompt: 

```shell
curl "http://127.0.0.1:8080/path?command"
```

$$ . $$

Second, with `POST` we register a new POST route for a handler function. The handler can receive a JSON body input and send specific messages to the tangle.
```go
webapi.Server().POST("path", HandlerFunc)
```	

For example, the following Handler `broadcastData` sends a data message to the tangle
```go
func broadcastData(c echo.Context) error {
	var request Request
	if err := c.Bind(&request); err != nil {
		log.Info(err.Error())
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}

	msg, err := messagelayer.IssuePayload(
		payload.NewGenericDataPayload(request.Data), messagelayer.Tangle())
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{ID: msg.ID().String()})
}
```
As an example the JSON body   
```json
{
	"data":"HelloWorld"
}
```
can be sent to `http://127.0.0.1:8080/data`, which will issue a data message containing "HelloWor" (note that in this  example the data input is size limited.)
 
