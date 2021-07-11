/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine http server
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package network

import (
	"bytes"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HttpClient .
type HttpClient struct {
	debug          bool
	httpConnection *HttpConnection
}

// NewHttpClient .
func NewHttpClient() *HttpClient {
	return &HttpClient{}
}

// SetDebug .
func (c *HttpClient) SetDebug(debug bool) {
	c.debug = debug
}

// Dial .
func (c *HttpClient) Dial(localAddress string, remoteAddress string, timeout time.Duration) error {
	tcpConnection, err := TcpConnect(localAddress, remoteAddress, timeout)
	if err != nil {
		return err
	}
	c.httpConnection = NewHttpConnection(tcpConnection)
	return nil
}

// ConnectTo .
func (c *HttpClient) ConnectTo(address string, timeout time.Duration) error {
	tcpConnection, err := TcpConnect("", address, timeout)
	if err != nil {
		return err
	}
	c.httpConnection = NewHttpConnection(tcpConnection)
	return nil
}

// Close .
func (c *HttpClient) Close() error {
	return c.httpConnection.Close()
}

// Send .
func (c *HttpClient) Send(request *HttpRequest, writeTimeout time.Duration) error {
	return c.httpConnection.WriteRequest(request, writeTimeout)
}

// Recv .
func (c *HttpClient) Recv(readTimeout time.Duration) (*HttpResponse, error) {
	return c.httpConnection.ReadResponse(readTimeout)
}

// Get .
func (c *HttpClient) Get(uri string, readTimeout time.Duration, writeTimeout time.Duration) (*HttpResponse, error) {
	request := NewHttpRequest()
	request.SetMethod("GET")
	request.SetUri(uri)
	request.SetVersion("HTTP/1.1")
	if c.debug {
		println(request.String())
	}
	if err := c.Send(request, writeTimeout); err != nil {
		return nil, err
	}
	return c.Recv(readTimeout)
}

// Post .
func (c *HttpClient) Post(uri string, posts map[string]string, readTimeout time.Duration, writeTimeout time.Duration) (*HttpResponse, error) {
	request := NewHttpRequest()
	request.SetMethod("POST")
	request.SetUri(uri)
	request.SetVersion("HTTP/1.1")
	request.SetPosts(posts)
	if c.debug {
		println(request.String())
	}
	if err := c.Send(request, writeTimeout); err != nil {
		return nil, err
	}
	return c.Recv(readTimeout)
}

const (
	httpHeadBodySeparator    = "\r\n\r\n"
	httpHeadBodySeparatorLen = len(httpHeadBodySeparator)
	httpHeaderPartSeparator  = " "
	httpHeaderPropSeparator  = ":"
	httpHeaderSeparator      = "\r\n"
)

// HttpConnection .
type HttpConnection struct {
	*TcpConnection        // 内嵌 *TcpConnection
	requestStream  []byte // 保存未读的请求数据流
	responseStream []byte // 保存未读的响应数据流
	headBuffer     []byte // 解析请求/响应的头部时使用的临时缓冲区
	bodyBuffer     []byte // 解析请求/响应的内容时使用的临时缓冲区
}

const (
	headBufferSize = 512
	bodyBufferSize = 1024
)

// NewHttpConnection .
func NewHttpConnection(tcpConnection *TcpConnection) *HttpConnection {
	return &HttpConnection{TcpConnection: tcpConnection, headBuffer: make([]byte, headBufferSize), bodyBuffer: make([]byte, bodyBufferSize)}
}

// ReadRequest .
func (c *HttpConnection) ReadRequest(timeout time.Duration) (*HttpRequest, error) {
	if err := c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	request, err := c.readMessageTo(NewHttpRequest(), &c.requestStream)
	if request == nil {
		return nil, err
	}
	// 获取 remote addr，客户端ip地址
	request.(*HttpRequest).remoteAddr = c.TCPConn.RemoteAddr().String()
	return request.(*HttpRequest), err
}

// ReadResponse .
func (c *HttpConnection) ReadResponse(timeout time.Duration) (*HttpResponse, error) {
	if err := c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	response, err := c.readMessageTo(NewHttpResponse(), &c.responseStream)
	if response == nil {
		return nil, err
	}
	return response.(*HttpResponse), err
}

// WriteRequest .
func (c *HttpConnection) WriteRequest(request *HttpRequest, timeout time.Duration) error {
	if err := c.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	return c.writeStream(request.Stream())
}

// WriteResponse .
func (c *HttpConnection) WriteResponse(response *HttpResponse, timeout time.Duration) error {
	if err := c.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	return c.writeStream(response.Stream())
}

// readMessageTo .
func (c *HttpConnection) readMessageTo(httpMessage HttpMessage, stream *[]byte) (HttpMessage, error) {
	for *stream == nil || bytes.Index(*stream, []byte(httpHeadBodySeparator)) == -1 {
		if err := c.readToStream(stream, c.headBuffer); err != nil {
			return nil, err
		}
	}
	position := bytes.Index(*stream, []byte(httpHeadBodySeparator))
	headStream := (*stream)[0:position]
	headLen := len(headStream)
	if len(*stream) > headLen+httpHeadBodySeparatorLen {
		*stream = (*stream)[position+httpHeadBodySeparatorLen:]
	} else {
		*stream = nil
	}
	if err := parseHeadStreamTo(httpMessage, headStream); err != nil {
		return nil, err
	}
	contentLength, err := strconv.Atoi(httpMessage.Header("Content-Length"))
	if err != nil {
		contentLength = 0
	}
	if contentLength > 1<<20 {
		return nil, errors.New("body too large")
	}
	if contentLength > 0 {
		for *stream == nil || len(*stream) < contentLength {
			if err := c.readToStream(stream, c.bodyBuffer); err != nil {
				return nil, err
			}
		}
		bodyStream := (*stream)[:contentLength]
		if err := httpMessage.parseBodyStream(bodyStream); err != nil {
			return nil, err
		}
		if len(*stream) == contentLength {
			*stream = nil
		} else {
			*stream = (*stream)[contentLength:]
		}
	}
	return httpMessage, nil
}

func (c *HttpConnection) writeStream(stream []byte) error {
	_, err := c.TcpConnection.Write(stream)
	return err
}

func (c *HttpConnection) readToStream(stream *[]byte, buffer []byte) error {
	n, err := c.TcpConnection.Read(buffer)
	if n > 0 {
		*stream = append(*stream, buffer[:n]...)
	}
	if err != nil {
		return err
	}
	return nil
}

// HttpMessage .
type HttpMessage interface {
	SetHeader(key string, value string)
	Header(key string) string
	Stream() []byte
	parseHairStream(hairStream []byte) error
	parseBodyStream(bodyStream []byte) error
}

func parseHeadStreamTo(message HttpMessage, headStream []byte) error {
	lines := bytes.Split(headStream, []byte(httpHeaderSeparator))
	lineCount := len(lines)
	if lineCount == 0 {
		return errors.New("bad http message")
	}
	message.parseHairStream(lines[0])
	if lineCount > 1 {
		lines = lines[1:]
		for _, line := range lines {
			parts := bytes.SplitN(line, []byte(httpHeaderPropSeparator), 2)
			if len(parts) != 2 {
				return errors.New("bad http message")
			}
			message.SetHeader(strings.Trim(string(parts[0]), " "), strings.Trim(string(parts[1]), " "))
		}
	}
	return nil
}

// HttpRequest .
type HttpRequest struct {
	remoteAddr string
	method     string
	uri        string
	version    string
	headers    map[string]string
	pathInfo   string
	gets       map[string]string
	posts      map[string]string
	cookies    map[string]string
	vars       map[string]interface{}
}

// NewHttpRequest .
func NewHttpRequest() *HttpRequest {
	return &HttpRequest{
		method:   "GET",
		uri:      "/",
		version:  "HTTP/1.1",
		pathInfo: "/",
		headers:  make(map[string]string),
		gets:     make(map[string]string),
		posts:    make(map[string]string),
		cookies:  make(map[string]string),
		vars:     make(map[string]interface{}),
	}
}

// GetRemoteIP .
func (r *HttpRequest) GetRemoteIP() string {
	parts := strings.SplitN(r.remoteAddr, ":", 2)
	return parts[0]
}

// Method .
func (r *HttpRequest) Method() string {
	return r.method
}

// SetMethod .
func (r *HttpRequest) SetMethod(method string) {
	r.method = method
}

// Uri .
func (r *HttpRequest) Uri() string {
	return r.uri
}

// SetUri .
func (r *HttpRequest) SetUri(uri string) {
	r.uri = uri
	if markPos := strings.Index(r.uri, "?"); markPos != -1 {
		r.pathInfo = r.uri[:markPos]
	}
}

// PathInfo .
func (r *HttpRequest) PathInfo() string {
	return r.pathInfo
}

// Version .
func (r *HttpRequest) Version() string {
	return r.version
}

// SetVersion .
func (r *HttpRequest) SetVersion(version string) {
	r.version = version
}

func (r *HttpRequest) IsGet() bool {
	return r.Method() == "GET"
}

func (r *HttpRequest) IsPost() bool {
	return r.Method() == "POST"
}

func (r *HttpRequest) IsPut() bool {
	return r.Method() == "PUT"
}

func (r *HttpRequest) IsDelete() bool {
	return r.Method() == "DELETE"
}

func (r *HttpRequest) Gets() map[string]string {
	return r.gets
}

func (r *HttpRequest) Ghas(key string) bool {
	if _, ok := r.gets[key]; ok {
		return true
	}
	return false
}

func (r *HttpRequest) Gstr(key string) string {
	if value, ok := r.gets[key]; ok {
		return value
	}
	return ""
}

func (r *HttpRequest) Gint(key string) int {
	value := r.Gstr(key)
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) Posts() map[string]string {
	return r.posts
}

func (r *HttpRequest) Phas(key string) bool {
	if _, ok := r.posts[key]; ok {
		return true
	}
	return false
}

func (r *HttpRequest) Pstr(key string) string {
	if value, ok := r.posts[key]; ok {
		return value
	}
	return ""
}

func (r *HttpRequest) Pint(key string) int {
	value := r.Pstr(key)
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) Rint(key string) int {
	if value := r.Pint(key); value != 0 {
		return value
	}
	return r.Gint(key)
}

func (r *HttpRequest) Rstr(key string) string {
	if value := r.Pstr(key); value != "" {
		return value
	}
	return r.Gstr(key)
}

func (r *HttpRequest) SetPosts(posts map[string]string) {
	r.posts = posts
}

func (r *HttpRequest) Cookies() map[string]string {
	return r.cookies
}

func (r *HttpRequest) Chas(key string) bool {
	if _, ok := r.cookies[key]; ok {
		return true
	}
	return false
}

func (r *HttpRequest) Cstr(key string) string {
	if value, ok := r.cookies[key]; ok {
		return value
	}
	return ""
}

func (r *HttpRequest) Cint(key string) int {
	value := r.Cstr(key)
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return i
}

func (r *HttpRequest) IsKeepAlive() bool {
	return r.version == "HTTP/1.1" && r.Header("Connection") != "close"
}

func (r *HttpRequest) Header(key string) string {
	if value, ok := r.headers[key]; ok {
		return value
	}
	return ""
}

func (r *HttpRequest) SetHeader(key string, value string) {
	r.headers[key] = value
}

func (r *HttpRequest) Var(key string) interface{} {
	return r.vars[key]
}

func (r *HttpRequest) SetVar(key string, value interface{}) {
	r.vars[key] = value
}

func (r *HttpRequest) Stream() []byte {
	body := ""
	if r.method == "POST" {
		postsLen := len(r.posts)
		if postsLen > 0 {
			pairs := make([]string, 0, postsLen)
			for key, value := range r.posts {
				pairs = append(pairs, url.QueryEscape(key)+"="+url.QueryEscape(value))
			}
			body = strings.Join(pairs, "&")
		}
		r.headers["Content-Length"] = strconv.Itoa(len(body))
	}
	// @todo: 需要优化
	s := r.method + httpHeaderPartSeparator + r.uri + httpHeaderPartSeparator + r.version + httpHeaderSeparator
	for key, value := range r.headers {
		s += key + httpHeaderPropSeparator + " " + value + httpHeaderSeparator
	}
	s += httpHeaderSeparator + body
	return []byte(s)
}

func (r *HttpRequest) String() string {
	return string(r.Stream())
}

func (r *HttpRequest) parseHairStream(hairStream []byte) error {
	parts := bytes.SplitN(hairStream, []byte(httpHeaderPartSeparator), 3)
	if len(parts) != 3 {
		return errors.New("bad http request")
	}
	r.method, r.uri, r.version = string(parts[0]), string(parts[1]), string(parts[2])
	r.pathInfo = r.uri
	if markPos := strings.Index(r.uri, "?"); markPos != -1 {
		r.pathInfo = r.uri[0:markPos]
		if markPos != len(r.uri)-1 {
			queryString := r.uri[markPos+1:]
			kvStrings := strings.Split(queryString, "&")
			for _, kvString := range kvStrings {
				parts := strings.SplitN(kvString, "=", 2)
				if len(parts) != 2 {
					return errors.New("bad http request")
				}
				key, err := url.QueryUnescape(parts[0])
				if err != nil {
					key = parts[0]
				}
				value, err := url.QueryUnescape(parts[1])
				if err != nil {
					value = parts[1]
				}
				r.gets[key] = value
			}
		}
	}
	return nil
}

func (r *HttpRequest) parseBodyStream(bodyStream []byte) error {
	kvStrings := strings.Split(string(bodyStream), "&")
	for _, kvString := range kvStrings {
		parts := strings.SplitN(kvString, "=", 2)
		if len(parts) != 2 {
			return errors.New("bad http request")
		}
		key, err := url.QueryUnescape(parts[0])
		if err != nil {
			key = parts[0]
		}
		value, err := url.QueryUnescape(parts[1])
		if err != nil {
			value = parts[1]
		}
		r.posts[key] = value
	}
	return nil
}

// HttpResponse .
type HttpResponse struct {
	version    string
	code       int
	phrase     string
	headers    map[string]string
	bodyStream []byte
}

var httpStatus = map[int]string{
	200: "OK",
	301: "Moved Permanently",
	302: "Moved Temporarily",
	400: "Bad Request",
	401: "Unauthorized",
	403: "Forbidden",
	404: "Not Found",
	500: "Internal Server Error",
}

// NewHttpResponse .
func NewHttpResponse() *HttpResponse {
	return &HttpResponse{version: "HTTP/1.1", code: 200, phrase: httpStatus[200], headers: map[string]string{
		"Server":       "Koala-Server",
		"Content-Type": "text/html; charset=utf-8",
		"Connection":   "keep-alive",
	}}
}

func (r *HttpResponse) Version() string {
	return r.version
}

func (r *HttpResponse) SetVersion(version string) {
	r.version = version
}

func (r *HttpResponse) Code() int {
	return r.code
}

func (r *HttpResponse) SetCode(code int) {
	r.code = code
	r.phrase = httpStatus[r.code]
}

func (r *HttpResponse) Status() (int, string) {
	return r.code, r.phrase
}

func (r *HttpResponse) SetStatus(code int, phrase string) {
	r.code, r.phrase = code, phrase
}

func (r *HttpResponse) Header(key string) string {
	if value, ok := r.headers[key]; ok {
		return value
	}
	return ""
}

func (r *HttpResponse) SetHeader(key string, value string) {
	r.headers[key] = value
}

func (r *HttpResponse) IsConnectionClose() bool {
	return r.Version() != "HTTP/1.1" || r.Header("Connection") == "close"
}

func (r *HttpResponse) SetBodyStream(bodyStream []byte) {
	r.bodyStream = bodyStream
}

func (r *HttpResponse) BodyStream() []byte {
	return r.bodyStream
}

func (r *HttpResponse) SetBodyString(bodyString string) {
	r.SetBodyStream([]byte(bodyString))
}

func (r *HttpResponse) BodyString() string {
	return string(r.bodyStream)
}

func (r *HttpResponse) Putb(stream []byte) {
	if len(stream) > 0 {
		r.bodyStream = append(r.bodyStream, stream...)
	}
}

func (r *HttpResponse) Puts(content string) {
	r.bodyStream = append(r.bodyStream, content...)
}

func (r *HttpResponse) Stream() []byte {
	var b bytes.Buffer
	b.WriteString(r.version)
	b.WriteString(httpHeaderPartSeparator)
	b.WriteString(strconv.Itoa(r.code))
	b.WriteString(httpHeaderPartSeparator)
	b.WriteString(r.phrase)
	b.WriteString(httpHeaderSeparator)
	r.headers["Content-Length"] = strconv.Itoa(len(r.bodyStream))
	for key, value := range r.headers {
		b.WriteString(key)
		b.WriteString(httpHeaderPropSeparator + " ")
		b.WriteString(value)
		b.WriteString(httpHeaderSeparator)
	}
	b.WriteString(httpHeaderSeparator)
	if len(r.bodyStream) > 0 {
		b.Write(r.bodyStream)
	}
	return b.Bytes()
}

func (r *HttpResponse) String() string {
	return string(r.Stream())
}

func (r *HttpResponse) parseHairStream(hairStream []byte) error {
	parts := bytes.SplitN(hairStream, []byte(httpHeaderPartSeparator), 3)
	if len(parts) != 3 {
		return errors.New("bad response")
	}
	r.version = string(parts[0])
	if code, err := strconv.Atoi(string(parts[1])); err == nil {
		r.code = code
	} else {
		r.code = 500
	}
	r.phrase = string(parts[2])
	return nil
}

func (r *HttpResponse) parseBodyStream(bodyStream []byte) error {
	r.bodyStream = bodyStream
	return nil
}
