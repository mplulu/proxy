package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/mplulu/log"
	"github.com/mplulu/renv"
	"github.com/mplulu/request_blocker/env"
	"moul.io/http2curl"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var ErrCallError = errors.New("request API Call Error")

type CallError struct {
	Url, Body, Resp string
	HTTPError       string
	Err             error
}

func (request *CallError) Error() string {
	str := strings.Builder{}
	str.WriteString(fmt.Sprintf("%v\n", request.Err.Error()))
	str.WriteString(fmt.Sprintf("Sent: %v\n", request.Url))
	str.WriteString(fmt.Sprintf("Body: %v\n", strings.TrimSpace(request.Body)))
	str.WriteString(fmt.Sprintf("Time: %v\n", time.Now()))
	if request.Resp != "" {
		str.WriteString(fmt.Sprintf("Received: %v\n", request.Resp))
	}
	if request.HTTPError != "" {
		str.WriteString(fmt.Sprintf("Received Error: %v\n", request.HTTPError))
	}
	return str.String()
}
func (e *CallError) Unwrap() error { return e.Err }

func NewCallError(httpErr error, url, body, resp string) error {
	return &CallError{
		Url:       url,
		Body:      body,
		HTTPError: httpErr.Error(),
		Resp:      resp,

		Err: ErrCallError,
	}
}

func main() {

	var envObjc *env.ENV
	renv.ParseCmd(&envObjc)
	env.E = envObjc

	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	e.HideBanner = true

	headersIgnore := []string{"Content-Length"}

	// ignoreLogPrefix := []string{}
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			// for _, prefix := range ignoreLogPrefix {
			// 	if strings.HasPrefix(c.Request().URL.Path, prefix) {
			// 		return true
			// 	}
			// }
			return false
		},
		CustomTimeFormat: "02/01/06T15:04:05Z",
		Format:           "[${host}]${remote_ip},t=${time_custom},d=${latency_human},s=${status},m=${method},uri=${uri}\n",
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	e.GET("/*", func(c echo.Context) error {
		var err error
		bodyBytes, err := ioutil.ReadAll(c.Request().Body)
		if err != nil {
			panic(err)
		}
		host := c.Request().Header.Get("X-TARGET")
		endpoint := fmt.Sprintf("%v%v", host, c.Request().URL.RequestURI())
		req, err := http.NewRequest(http.MethodGet, endpoint, bytes.NewBuffer(bodyBytes))
		if err != nil {
			err = NewCallError(err, endpoint, string(bodyBytes), "")
			panic(err)
		}
		for key, value := range c.Request().Header {
			if !ItemExists(headersIgnore, key) && len(value) > 0 {
				req.Header.Set(key, value[0])
			}
		}

		command, _ := http2curl.GetCurlCommand(req)
		log.Log(command.String())

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			err = NewCallError(err, endpoint, string(bodyBytes), "")
			panic(err)
		}
		defer resp.Body.Close()
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = NewCallError(err, endpoint, string(bodyBytes), string(respBytes))
			panic(err)
		}
		return c.HTML(http.StatusOK, string(respBytes))
	})

	go e.Logger.Fatal(e.Start(envObjc.Host))
	select {}
}

func ItemExists(arrayType interface{}, item interface{}) bool {
	arr := reflect.ValueOf(arrayType)

	if arr.Kind() != reflect.Slice && arr.Kind() != reflect.Array {
		panic(fmt.Sprintf("Invalid data-type: %s", arr.Kind()))
	}

	for i := 0; i < arr.Len(); i++ {
		if arr.Index(i).Interface() == item {
			return true
		}
	}

	return false
}
