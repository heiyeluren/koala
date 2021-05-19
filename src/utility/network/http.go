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

type HttpClient struct {
    debug          bool
    httpConnection *HttpConnection
}

func NewHttpClient() *HttpClient {
    return &HttpClient{}
}

func (this *HttpClient) SetDebug(debug bool) {
    this.debug = debug
}

func (this *HttpClient) Dial(localAddress string, remoteAddress string, timeout time.Duration) error {
    tcpConnection, err := TcpConnect(localAddress, remoteAddress, timeout)
    if err != nil {
        return err
    }
    this.httpConnection = NewHttpConnection(tcpConnection)
    return nil
}

func (this *HttpClient) ConnectTo(address string, timeout time.Duration) error {
    tcpConnection, err := TcpConnect("", address, timeout)
    if err != nil {
        return err
    }
    this.httpConnection = NewHttpConnection(tcpConnection)
    return nil
}

func (this *HttpClient) Close() error {
    return this.httpConnection.Close()
}

func (this *HttpClient) Send(request *HttpRequest, writeTimeout time.Duration) error {
    return this.httpConnection.WriteRequest(request, writeTimeout)
}

func (this *HttpClient) Recv(readTimeout time.Duration) (*HttpResponse, error) {
    return this.httpConnection.ReadResponse(readTimeout)
}

func (this *HttpClient) Get(uri string, readTimeout time.Duration, writeTimeout time.Duration) (*HttpResponse, error) {
    request := NewHttpRequest()
    request.SetMethod("GET")
    request.SetUri(uri)
    request.SetVersion("HTTP/1.1")
    if this.debug {
        println(request.String())
    }
    if err := this.Send(request, writeTimeout); err != nil {
        return nil, err
    }
    return this.Recv(readTimeout)
}

func (this *HttpClient) Post(uri string, posts map[string]string, readTimeout time.Duration, writeTimeout time.Duration) (*HttpResponse, error) {
    request := NewHttpRequest()
    request.SetMethod("POST")
    request.SetUri(uri)
    request.SetVersion("HTTP/1.1")
    request.SetPosts(posts)
    if this.debug {
        println(request.String())
    }
    if err := this.Send(request, writeTimeout); err != nil {
        return nil, err
    }
    return this.Recv(readTimeout)
}

const (
    httpHeadBodySeparator    = "\r\n\r\n"
    httpHeadBodySeparatorLen = len(httpHeadBodySeparator)
    httpHeaderPartSeparator  = " "
    httpHeaderPropSeparator  = ":"
    httpHeaderSeparator      = "\r\n"
)

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

func NewHttpConnection(tcpConnection *TcpConnection) *HttpConnection {
    return &HttpConnection{TcpConnection: tcpConnection, headBuffer: make([]byte, headBufferSize), bodyBuffer: make([]byte, bodyBufferSize)}
}

func (this *HttpConnection) ReadRequest(timeout time.Duration) (*HttpRequest, error) {
    if err := this.SetReadDeadline(time.Now().Add(timeout)); err != nil {
        return nil, err
    }
    request, err := this.readMessageTo(NewHttpRequest(), &this.requestStream)
    if request == nil {
        return nil, err
    }
    // 获取 remote addr，客户端ip地址
    request.(*HttpRequest).remoteAddr = this.TCPConn.RemoteAddr().String()
    return request.(*HttpRequest), err
}

func (this *HttpConnection) ReadResponse(timeout time.Duration) (*HttpResponse, error) {
    if err := this.SetReadDeadline(time.Now().Add(timeout)); err != nil {
        return nil, err
    }
    response, err := this.readMessageTo(NewHttpResponse(), &this.responseStream)
    if response == nil {
        return nil, err
    }
    return response.(*HttpResponse), err
}

func (this *HttpConnection) WriteRequest(request *HttpRequest, timeout time.Duration) error {
    if err := this.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
        return err
    }
    return this.writeStream(request.Stream())
}

func (this *HttpConnection) WriteResponse(response *HttpResponse, timeout time.Duration) error {
    if err := this.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
        return err
    }
    return this.writeStream(response.Stream())
}

func (this *HttpConnection) readMessageTo(httpMessage HttpMessage, stream *[]byte) (HttpMessage, error) {
    for *stream == nil || bytes.Index(*stream, []byte(httpHeadBodySeparator)) == -1 {
        if err := this.readToStream(stream, this.headBuffer); err != nil {
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
        return nil, errors.New("body too large!")
    }
    if contentLength > 0 {
        for *stream == nil || len(*stream) < contentLength {
            if err := this.readToStream(stream, this.bodyBuffer); err != nil {
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

func (this *HttpConnection) writeStream(stream []byte) error {
    _, err := this.TcpConnection.Write(stream)
    return err
}

func (this *HttpConnection) readToStream(stream *[]byte, buffer []byte) error {
    n, err := this.TcpConnection.Read(buffer)
    if n > 0 {
        *stream = append(*stream, buffer[:n]...)
    }
    if err != nil {
        return err
    }
    return nil
}

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
        return errors.New("Bad http message")
    }
    message.parseHairStream(lines[0])
    if lineCount > 1 {
        lines = lines[1:]
        for _, line := range lines {
            parts := bytes.SplitN(line, []byte(httpHeaderPropSeparator), 2)
            if len(parts) != 2 {
                return errors.New("Bad http message")
            }
            message.SetHeader(strings.Trim(string(parts[0]), " "), strings.Trim(string(parts[1]), " "))
        }
    }
    return nil
}

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

func (this *HttpRequest) GetRemoteIP() string {
    parts := strings.SplitN(this.remoteAddr, ":", 2)
    return parts[0]
}

func (this *HttpRequest) Method() string {
    return this.method
}

func (this *HttpRequest) SetMethod(method string) {
    this.method = method
}

func (this *HttpRequest) Uri() string {
    return this.uri
}

func (this *HttpRequest) SetUri(uri string) {
    this.uri = uri
    if markPos := strings.Index(this.uri, "?"); markPos != -1 {
        this.pathInfo = this.uri[:markPos]
    }
}

func (this *HttpRequest) PathInfo() string {
    return this.pathInfo
}

func (this *HttpRequest) Version() string {
    return this.version
}

func (this *HttpRequest) SetVersion(version string) {
    this.version = version
}

func (this *HttpRequest) IsGet() bool {
    return this.Method() == "GET"
}

func (this *HttpRequest) IsPost() bool {
    return this.Method() == "POST"
}

func (this *HttpRequest) IsPut() bool {
    return this.Method() == "PUT"
}

func (this *HttpRequest) IsDelete() bool {
    return this.Method() == "DELETE"
}

func (this *HttpRequest) Gets() map[string]string {
    return this.gets
}

func (this *HttpRequest) Ghas(key string) bool {
    if _, ok := this.gets[key]; ok {
        return true
    }
    return false
}

func (this *HttpRequest) Gstr(key string) string {
    if value, ok := this.gets[key]; ok {
        return value
    }
    return ""
}

func (this *HttpRequest) Gint(key string) int {
    value := this.Gstr(key)
    if value == "" {
        return 0
    }
    i, err := strconv.Atoi(value)
    if err != nil {
        return 0
    }
    return i
}

func (this *HttpRequest) Posts() map[string]string {
    return this.posts
}

func (this *HttpRequest) Phas(key string) bool {
    if _, ok := this.posts[key]; ok {
        return true
    }
    return false
}

func (this *HttpRequest) Pstr(key string) string {
    if value, ok := this.posts[key]; ok {
        return value
    }
    return ""
}

func (this *HttpRequest) Pint(key string) int {
    value := this.Pstr(key)
    if value == "" {
        return 0
    }
    i, err := strconv.Atoi(value)
    if err != nil {
        return 0
    }
    return i
}

func (this *HttpRequest) Rint(key string) int {
    if value := this.Pint(key); value != 0 {
        return value
    }
    return this.Gint(key)
}

func (this *HttpRequest) Rstr(key string) string {
    if value := this.Pstr(key); value != "" {
        return value
    }
    return this.Gstr(key)
}

func (this *HttpRequest) SetPosts(posts map[string]string) {
    this.posts = posts
}

func (this *HttpRequest) Cookies() map[string]string {
    return this.cookies
}

func (this *HttpRequest) Chas(key string) bool {
    if _, ok := this.cookies[key]; ok {
        return true
    }
    return false
}

func (this *HttpRequest) Cstr(key string) string {
    if value, ok := this.cookies[key]; ok {
        return value
    }
    return ""
}

func (this *HttpRequest) Cint(key string) int {
    value := this.Cstr(key)
    if value == "" {
        return 0
    }
    i, err := strconv.Atoi(value)
    if err != nil {
        return 0
    }
    return i
}

func (this *HttpRequest) IsKeepAlive() bool {
    return this.version == "HTTP/1.1" && this.Header("Connection") != "close"
}

func (this *HttpRequest) Header(key string) string {
    if value, ok := this.headers[key]; ok {
        return value
    }
    return ""
}

func (this *HttpRequest) SetHeader(key string, value string) {
    this.headers[key] = value
}

func (this *HttpRequest) Var(key string) interface{} {
    return this.vars[key]
}

func (this *HttpRequest) SetVar(key string, value interface{}) {
    this.vars[key] = value
}

func (this *HttpRequest) Stream() []byte {
    body := ""
    if this.method == "POST" {
        postsLen := len(this.posts)
        if postsLen > 0 {
            pairs := make([]string, 0, postsLen)
            for key, value := range this.posts {
                pairs = append(pairs, url.QueryEscape(key)+"="+url.QueryEscape(value))
            }
            body = strings.Join(pairs, "&")
        }
        this.headers["Content-Length"] = strconv.Itoa(len(body))
    }
    // @todo: 需要优化
    s := this.method + httpHeaderPartSeparator + this.uri + httpHeaderPartSeparator + this.version + httpHeaderSeparator
    for key, value := range this.headers {
        s += key + httpHeaderPropSeparator + " " + value + httpHeaderSeparator
    }
    s += httpHeaderSeparator + body
    return []byte(s)
}

func (this *HttpRequest) String() string {
    return string(this.Stream())
}

func (this *HttpRequest) parseHairStream(hairStream []byte) error {
    parts := bytes.SplitN(hairStream, []byte(httpHeaderPartSeparator), 3)
    if len(parts) != 3 {
        return errors.New("Bad http request")
    }
    this.method, this.uri, this.version = string(parts[0]), string(parts[1]), string(parts[2])
    this.pathInfo = this.uri
    if markPos := strings.Index(this.uri, "?"); markPos != -1 {
        this.pathInfo = this.uri[0:markPos]
        if markPos != len(this.uri)-1 {
            queryString := this.uri[markPos+1:]
            kvStrings := strings.Split(queryString, "&")
            for _, kvString := range kvStrings {
                parts := strings.SplitN(kvString, "=", 2)
                if len(parts) != 2 {
                    return errors.New("Bad http request")
                }
                key, err := url.QueryUnescape(parts[0])
                if err != nil {
                    key = parts[0]
                }
                value, err := url.QueryUnescape(parts[1])
                if err != nil {
                    value = parts[1]
                }
                this.gets[key] = value
            }
        }
    }
    return nil
}

func (this *HttpRequest) parseBodyStream(bodyStream []byte) error {
    kvStrings := strings.Split(string(bodyStream), "&")
    for _, kvString := range kvStrings {
        parts := strings.SplitN(kvString, "=", 2)
        if len(parts) != 2 {
            return errors.New("Bad http request")
        }
        key, err := url.QueryUnescape(parts[0])
        if err != nil {
            key = parts[0]
        }
        value, err := url.QueryUnescape(parts[1])
        if err != nil {
            value = parts[1]
        }
        this.posts[key] = value
    }
    return nil
}

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

func NewHttpResponse() *HttpResponse {
    return &HttpResponse{version: "HTTP/1.1", code: 200, phrase: httpStatus[200], headers: map[string]string{
        "Server":       "Koala-Server",
        "Content-Type": "text/html; charset=utf-8",
        "Connection":   "keep-alive",
    }}
}

func (this *HttpResponse) Version() string {
    return this.version
}

func (this *HttpResponse) SetVersion(version string) {
    this.version = version
}

func (this *HttpResponse) Code() int {
    return this.code
}

func (this *HttpResponse) SetCode(code int) {
    this.code = code
    this.phrase = httpStatus[this.code]
}

func (this *HttpResponse) Status() (int, string) {
    return this.code, this.phrase
}

func (this *HttpResponse) SetStatus(code int, phrase string) {
    this.code, this.phrase = code, phrase
}

func (this *HttpResponse) Header(key string) string {
    if value, ok := this.headers[key]; ok {
        return value
    }
    return ""
}

func (this *HttpResponse) SetHeader(key string, value string) {
    this.headers[key] = value
}

func (this *HttpResponse) IsConnectionClose() bool {
    return this.Version() != "HTTP/1.1" || this.Header("Connection") == "close"
}

func (this *HttpResponse) SetBodyStream(bodyStream []byte) {
    this.bodyStream = bodyStream
}

func (this *HttpResponse) BodyStream() []byte {
    return this.bodyStream
}

func (this *HttpResponse) SetBodyString(bodyString string) {
    this.SetBodyStream([]byte(bodyString))
}

func (this *HttpResponse) BodyString() string {
    return string(this.bodyStream)
}

func (this *HttpResponse) Putb(stream []byte) {
    if len(stream) > 0 {
        this.bodyStream = append(this.bodyStream, stream...)
    }
}

func (this *HttpResponse) Puts(content string) {
    this.bodyStream = append(this.bodyStream, content...)
}

func (this *HttpResponse) Stream() []byte {
    var b bytes.Buffer
    b.WriteString(this.version)
    b.WriteString(httpHeaderPartSeparator)
    b.WriteString(strconv.Itoa(this.code))
    b.WriteString(httpHeaderPartSeparator)
    b.WriteString(this.phrase)
    b.WriteString(httpHeaderSeparator)
    this.headers["Content-Length"] = strconv.Itoa(len(this.bodyStream))
    for key, value := range this.headers {
        b.WriteString(key)
        b.WriteString(httpHeaderPropSeparator + " ")
        b.WriteString(value)
        b.WriteString(httpHeaderSeparator)
    }
    b.WriteString(httpHeaderSeparator)
    if len(this.bodyStream) > 0 {
        b.Write(this.bodyStream)
    }
    return b.Bytes()
}

func (this *HttpResponse) String() string {
    return string(this.Stream())
}

func (this *HttpResponse) parseHairStream(hairStream []byte) error {
    parts := bytes.SplitN(hairStream, []byte(httpHeaderPartSeparator), 3)
    if len(parts) != 3 {
        return errors.New("Bad response")
    }
    this.version = string(parts[0])
    if code, err := strconv.Atoi(string(parts[1])); err == nil {
        this.code = code
    } else {
        this.code = 500
    }
    this.phrase = string(parts[2])
    return nil
}

func (this *HttpResponse) parseBodyStream(bodyStream []byte) error {
    this.bodyStream = bodyStream
    return nil
}
