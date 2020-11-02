package blademaster

import (
	"context"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/djienet/kratos/pkg/ecode"
	"github.com/djienet/kratos/pkg/net/http/blademaster/binding"
	"github.com/djienet/kratos/pkg/net/http/blademaster/render"
	"github.com/djienet/kratos/pkg/net/metadata"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
)

const (
	_abortIndex int8 = math.MaxInt8 / 2
)

var (
	_openParen  = []byte("(")
	_closeParen = []byte(")")
)

// Context is the most important part. It allows us to pass variables between
// middleware, manage the flow, validate the JSON of a request and render a
// JSON response for example.
type Context struct {
	context.Context

	writermem responseWriter
	Request   *http.Request
	Writer    ResponseWriter

	// flow control
	index    int8
	handlers HandlersChain

	// Keys is a key/value pair exclusively for the context of each request.
	Keys map[string]interface{}
	// This mutex protect Keys map
	keysMutex sync.RWMutex

	Error error

	method string
	engine *Engine

	RoutePath string

	Params Params
}

/************************************/
/********** CONTEXT CREATION ********/
/************************************/

func (c *Context) reset() {
	c.Context = nil
	c.Writer = &c.writermem
	c.Params = c.Params[0:0]
	c.handlers = nil
	c.index = -1

	c.Keys = nil
	c.Error = nil
	c.method = ""
	c.RoutePath = ""
}

// Copy returns a copy of the current context that can be safely used outside the request's scope.
// This have to be used then the context has to be passed to a goroutine.
func (c *Context) Copy() *Context {
	cp := Context{
		writermem: c.writermem,
		Request:   c.Request,
		Params:    c.Params,
		engine:    c.engine,
	}
	cp.writermem.ResponseWriter = nil
	cp.Writer = &cp.writermem
	cp.index = _abortIndex
	cp.handlers = nil
	cp.Keys = map[string]interface{}{}
	for k, v := range c.Keys {
		cp.Keys[k] = v
	}
	paramCopy := make([]Param, len(cp.Params))
	copy(paramCopy, cp.Params)
	cp.Params = paramCopy
	return &cp
}

func (c *Context) GetIndex() int8 {
	return c.index
}

// HandlerName returns the main handler's name. For example if the handler is "handleGetUsers()", this
// function will return "main.handleGetUsers"
func (c *Context) HandlerName() string {
	return nameOfFunction(c.handlers.Last())
}

// Handler returns the main handler.
func (c *Context) Handler() HandlerFunc {
	return c.handlers.Last()
}

/************************************/
/*********** FLOW CONTROL ***********/
/************************************/

// Next should be used only inside middleware.
// It executes the pending handlers in the chain inside the calling handler.
// See example in godoc.
func (c *Context) Next() {
	c.index++
	s := int8(len(c.handlers))
	for ; c.index < s; c.index++ {
		c.handlers[c.index](c)
	}
}

// Abort prevents pending handlers from being called. Note that this will not stop the current handler.
// Let's say you have an authorization middleware that validates that the current request is authorized.
// If the authorization fails (ex: the password does not match), call Abort to ensure the remaining handlers
// for this request are not called.
func (c *Context) Abort() {
	c.index = _abortIndex
}

// AbortWithStatus calls `Abort()` and writes the headers with the specified status code.
// For example, a failed attempt to authenticate a request could use: context.AbortWithStatus(401).
func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
}

// IsAborted returns true if the current context was aborted.
func (c *Context) IsAborted() bool {
	return c.index >= _abortIndex
}

/************************************/
/******** METADATA MANAGEMENT********/
/************************************/

// Set is used to store a new key/value pair exclusively for this context.
// It also lazy initializes  c.Keys if it was not used previously.
func (c *Context) Set(key string, value interface{}) {
	c.keysMutex.Lock()
	if c.Keys == nil {
		c.Keys = make(map[string]interface{})
	}
	c.Keys[key] = value
	c.keysMutex.Unlock()
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exists it returns (nil, false)
func (c *Context) Get(key string) (value interface{}, exists bool) {
	if c.Keys != nil {
		c.keysMutex.RLock()
		value, exists = c.Keys[key]
		c.keysMutex.RUnlock()
	}
	return
}

// GetString returns the value associated with the key as a string.
func (c *Context) GetString(key string) (s string) {
	if val, ok := c.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}

// GetBool returns the value associated with the key as a boolean.
func (c *Context) GetBool(key string) (b bool) {
	if val, ok := c.Get(key); ok && val != nil {
		b, _ = val.(bool)
	}
	return
}

// GetInt returns the value associated with the key as an integer.
func (c *Context) GetInt(key string) (i int) {
	if val, ok := c.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}

// GetUint returns the value associated with the key as an unsigned integer.
func (c *Context) GetUint(key string) (ui uint) {
	if val, ok := c.Get(key); ok && val != nil {
		ui, _ = val.(uint)
	}
	return
}

// GetInt64 returns the value associated with the key as an integer.
func (c *Context) GetInt64(key string) (i64 int64) {
	if val, ok := c.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}

// GetUint64 returns the value associated with the key as an unsigned integer.
func (c *Context) GetUint64(key string) (ui64 uint64) {
	if val, ok := c.Get(key); ok && val != nil {
		ui64, _ = val.(uint64)
	}
	return
}

// GetFloat64 returns the value associated with the key as a float64.
func (c *Context) GetFloat64(key string) (f64 float64) {
	if val, ok := c.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}

/************************************/
/************ INPUT DATA ************/
/************************************/

// Param returns the value of the URL param.
// It is a shortcut for c.Params.ByName(key)
//		router.GET("/user/:id", func(c *gin.Context) {
//			// a GET request to /user/john
//			id := c.Param("id") // id == "john"
//		})
func (c *Context) Param(key string) string {
	return c.Params.ByName(key)
}

// Query returns the keyed url query value if it exists,
// othewise it returns an empty string `("")`.
// It is shortcut for `c.Request.URL.Query().Get(key)`
// 		GET /path?id=1234&name=Manu&value=
// 		c.Query("id") == "1234"
// 		c.Query("name") == "Manu"
// 		c.Query("value") == ""
// 		c.Query("wtf") == ""
func (c *Context) Query(key string) string {
	value, _ := c.GetQuery(key)
	return value
}

// DefaultQuery returns the keyed url query value if it exists,
// othewise it returns the specified defaultValue string.
// See: Query() and GetQuery() for further information.
// 		GET /?name=Manu&lastname=
// 		c.DefaultQuery("name", "unknown") == "Manu"
// 		c.DefaultQuery("id", "none") == "none"
// 		c.DefaultQuery("lastname", "none") == ""
func (c *Context) DefaultQuery(key, defaultValue string) string {
	if value, ok := c.GetQuery(key); ok {
		return value
	}
	return defaultValue
}

// GetQuery is like Query(), it returns the keyed url query value
// if it exists `(value, true)` (even when the value is an empty string),
// othewise it returns `("", false)`.
// It is shortcut for `c.Request.URL.Query().Get(key)`
// 		GET /?name=Manu&lastname=
// 		("Manu", true) == c.GetQuery("name")
// 		("", false) == c.GetQuery("id")
// 		("", true) == c.GetQuery("lastname")
func (c *Context) GetQuery(key string) (string, bool) {
	if values, ok := c.GetQueryArray(key); ok {
		return values[0], ok
	}
	return "", false
}

// QueryArray returns a slice of strings for a given query key.
// The length of the slice depends on the number of params with the given key.
func (c *Context) QueryArray(key string) []string {
	values, _ := c.GetQueryArray(key)
	return values
}

// GetQueryArray returns a slice of strings for a given query key, plus
// a boolean value whether at least one value exists for the given key.
func (c *Context) GetQueryArray(key string) ([]string, bool) {
	req := c.Request
	if values, ok := req.URL.Query()[key]; ok && len(values) > 0 {
		return values, true
	}
	return []string{}, false
}

// PostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns an empty string `("")`.
func (c *Context) PostForm(key string) string {
	value, _ := c.GetPostForm(key)
	return value
}

// DefaultPostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns the specified defaultValue string.
// See: PostForm() and GetPostForm() for further information.
func (c *Context) DefaultPostForm(key, defaultValue string) string {
	if value, ok := c.GetPostForm(key); ok {
		return value
	}
	return defaultValue
}

// GetPostForm is like PostForm(key). It returns the specified key from a POST urlencoded
// form or multipart form when it exists `(value, true)` (even when the value is an empty string),
// otherwise it returns ("", false).
// For example, during a PATCH request to update the user's email:
// 		email=mail@example.com  -->  ("mail@example.com", true) := GetPostForm("email") // set email to "mail@example.com"
// 		email=  			  	-->  ("", true) := GetPostForm("email") // set email to ""
//							 	-->  ("", false) := GetPostForm("email") // do nothing with email
func (c *Context) GetPostForm(key string) (string, bool) {
	if values, ok := c.GetPostFormArray(key); ok {
		return values[0], ok
	}
	return "", false
}

// PostFormArray returns a slice of strings for a given form key.
// The length of the slice depends on the number of params with the given key.
func (c *Context) PostFormArray(key string) []string {
	values, _ := c.GetPostFormArray(key)
	return values
}

// GetPostFormArray returns a slice of strings for a given form key, plus
// a boolean value whether at least one value exists for the given key.
func (c *Context) GetPostFormArray(key string) ([]string, bool) {
	req := c.Request
	req.ParseForm()
	req.ParseMultipartForm(32 << 20) // 32 MB
	if values := req.PostForm[key]; len(values) > 0 {
		return values, true
	}
	if req.MultipartForm != nil && req.MultipartForm.File != nil {
		if values := req.MultipartForm.Value[key]; len(values) > 0 {
			return values, true
		}
	}
	return []string{}, false
}

// ClientIP implements a best effort algorithm to return the real client IP, it parses
// X-Real-IP and X-Forwarded-For in order to work properly with reverse-proxies such us: nginx or haproxy.
// Use X-Forwarded-For before X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
func (c *Context) ClientIP() string {
	if c.engine.ForwardedByClientIP {
		clientIP := c.requestHeader("X-Forwarded-For")
		if index := strings.IndexByte(clientIP, ','); index >= 0 {
			clientIP = clientIP[0:index]
		}
		clientIP = strings.TrimSpace(clientIP)
		if clientIP != "" {
			return clientIP
		}
		clientIP = strings.TrimSpace(c.requestHeader("X-Real-Ip"))
		if clientIP != "" {
			return clientIP
		}
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr)); err == nil {
		return ip
	}

	return ""
}

// ContentType returns the Content-Type header of the request.
func (c *Context) ContentType() string {
	return filterFlags(c.requestHeader("Content-Type"))
}

// IsWebsocket returns true if the request headers indicate that a websocket
// handshake is being initiated by the client.
func (c *Context) IsWebsocket() bool {
	if strings.Contains(strings.ToLower(c.requestHeader("Connection")), "upgrade") &&
		strings.ToLower(c.requestHeader("Upgrade")) == "websocket" {
		return true
	}
	return false
}

func (c *Context) requestHeader(key string) string {
	return c.Request.Header.Get(key)
}

/************************************/
/******** RESPONSE RENDERING ********/
/************************************/

// bodyAllowedForStatus is a copy of http.bodyAllowedForStatus non-exported function.
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == 204:
		return false
	case status == 304:
		return false
	}
	return true
}

// Status sets the HTTP response code.
func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

// Header is a intelligent shortcut for c.Writer.Header().Set(key, value).
// It writes a header in the response.
// If value == "", this method removes the header `c.Writer.Header().Del(key)`
func (c *Context) Header(key, value string) {
	if value == "" {
		c.Writer.Header().Del(key)
	} else {
		c.Writer.Header().Set(key, value)
	}
}

// GetHeader returns value from request headers.
func (c *Context) GetHeader(key string) string {
	return c.requestHeader(key)
}

// GetRawData return stream data.
func (c *Context) GetRawData() ([]byte, error) {
	return ioutil.ReadAll(c.Request.Body)
}

// Render http response with http code by a render instance.
func (c *Context) Render(code int, r render.Render) {
	r.WriteContentType(c.Writer)
	c.Status(code)

	if !bodyAllowedForStatus(code) {
		return
	}

	params := c.Request.Form
	cb := template.JSEscapeString(params.Get("callback"))
	jsonp := cb != ""
	if jsonp {
		c.Writer.Write([]byte(cb))
		c.Writer.Write(_openParen)
	}
	if err := r.Render(c.Writer); err != nil {
		c.Error = err
		return
	}

	if jsonp {
		if _, err := c.Writer.Write(_closeParen); err != nil {
			c.Error = errors.WithStack(err)
		}
	}
}

// JSON serializes the given struct as JSON into the response body.
// It also sets the Content-Type as "application/json".
func (c *Context) JSON(data interface{}, err error) {
	code := http.StatusOK
	c.Error = err
	bcode := ecode.Cause(err)
	// TODO app allow 5xx?
	/*
		if bcode.Code() == -500 {
			code = http.StatusServiceUnavailable
		}
	*/
	writeStatusCode(c.Writer, bcode.Code())
	c.Render(code, render.JSON{
		Code:    bcode.Code(),
		Message: bcode.Message(),
		Data:    data,
	})
}

// JSONMap serializes the given map as map JSON into the response body.
// It also sets the Content-Type as "application/json".
func (c *Context) JSONMap(data map[string]interface{}, err error) {
	code := http.StatusOK
	c.Error = err
	bcode := ecode.Cause(err)
	// TODO app allow 5xx?
	/*
		if bcode.Code() == -500 {
			code = http.StatusServiceUnavailable
		}
	*/
	writeStatusCode(c.Writer, bcode.Code())
	data["status"] = bcode.Code()
	if _, ok := data["message"]; !ok {
		data["message"] = bcode.Message()
	}
	c.Render(code, render.MapJSON(data))
}

// XML serializes the given struct as XML into the response body.
// It also sets the Content-Type as "application/xml".
func (c *Context) XML(data interface{}, err error) {
	code := http.StatusOK
	c.Error = err
	bcode := ecode.Cause(err)
	// TODO app allow 5xx?
	/*
		if bcode.Code() == -500 {
			code = http.StatusServiceUnavailable
		}
	*/
	writeStatusCode(c.Writer, bcode.Code())
	c.Render(code, render.XML{
		Code:    bcode.Code(),
		Message: bcode.Message(),
		Data:    data,
	})
}

// Protobuf serializes the given struct as PB into the response body.
// It also sets the ContentType as "application/x-protobuf".
func (c *Context) Protobuf(data proto.Message, err error) {
	var (
		bytes []byte
	)

	code := http.StatusOK
	c.Error = err
	bcode := ecode.Cause(err)

	any := new(types.Any)
	if data != nil {
		if bytes, err = proto.Marshal(data); err != nil {
			c.Error = errors.WithStack(err)
			return
		}
		any.TypeUrl = "type.googleapis.com/" + proto.MessageName(data)
		any.Value = bytes
	}
	writeStatusCode(c.Writer, bcode.Code())
	c.Render(code, render.PB{
		Code:    int64(bcode.Code()),
		Message: bcode.Message(),
		Data:    any,
	})
}

// Bytes writes some data into the body stream and updates the HTTP code.
func (c *Context) Bytes(code int, contentType string, data ...[]byte) {
	c.Render(code, render.Data{
		ContentType: contentType,
		Data:        data,
	})
}

// String writes the given string into the response body.
func (c *Context) String(code int, format string, values ...interface{}) {
	c.Render(code, render.String{Format: format, Data: values})
}

// Redirect returns a HTTP redirect to the specific location.
func (c *Context) Redirect(code int, location string) {
	c.Render(-1, render.Redirect{
		Code:     code,
		Location: location,
		Request:  c.Request,
	})
}

// BindWith bind req arg with parser.
func (c *Context) BindWith(obj interface{}, b binding.Binding) error {
	return c.mustBindWith(obj, b)
}

// Bind checks the Content-Type to select a binding engine automatically,
// Depending the "Content-Type" header different bindings are used:
//     "application/json" --> JSON binding
//     "application/xml"  --> XML binding
// otherwise --> returns an error.
// It parses the request's body as JSON if Content-Type == "application/json" using JSON or XML as a JSON input.
// It decodes the json payload into the struct specified as a pointer.
// It writes a 400 error and sets Content-Type header "text/plain" in the response if input is not valid.
func (c *Context) Bind(obj interface{}) error {
	b := binding.Default(c.Request.Method, c.Request.Header.Get("Content-Type"))
	return c.mustBindWith(obj, b)
}

// mustBindWith binds the passed struct pointer using the specified binding engine.
// It will abort the request with HTTP 400 if any error ocurrs.
// See the binding package.
func (c *Context) mustBindWith(obj interface{}, b binding.Binding) (err error) {
	if err = b.Bind(c.Request, obj); err != nil {
		c.Error = ecode.RequestErr
		c.Render(http.StatusOK, render.JSON{
			Code:    ecode.RequestErr.Code(),
			Message: err.Error(),
			Data:    nil,
		})
		c.Abort()
	}
	return
}

func writeStatusCode(w http.ResponseWriter, ecode int) {
	header := w.Header()
	header.Set("kratos-status-code", strconv.FormatInt(int64(ecode), 10))
}

// RemoteIP implements a best effort algorithm to return the real client IP, it parses
// X-Real-IP and X-Forwarded-For in order to work properly with reverse-proxies such us: nginx or haproxy.
// Use X-Forwarded-For before X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
// Notice: metadata.RemoteIP take precedence over X-Forwarded-For and X-Real-Ip
func (c *Context) RemoteIP() (remoteIP string) {
	remoteIP = metadata.String(c, metadata.RemoteIP)
	if remoteIP != "" {
		return
	}

	remoteIP = c.Request.Header.Get("X-Forwarded-For")
	remoteIP = strings.TrimSpace(strings.Split(remoteIP, ",")[0])
	if remoteIP == "" {
		remoteIP = strings.TrimSpace(c.Request.Header.Get("X-Real-Ip"))
	}

	return
}
