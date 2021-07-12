/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine - Http server front api
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package koala

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/heiyeluren/koala/utility"
)

var frontPattern = regexp.MustCompile("^[a-z][0-9a-z_]*/[a-z][0-9a-z_]*$")

// FrontServer .
type FrontServer struct{}

// NewFrontServer .
func NewFrontServer() *FrontServer {
	return &FrontServer{}
}

// FrontListen .
func FrontListen() {
	tcpListener, err := utility.TcpListen(Config.Get("listen"))
	if err != nil {
		panic(err.Error())
	}
	for {
		tcpConnection, err := tcpListener.Accept()
		if err != nil {
			continue
		}
		go FrontDispatch(utility.NewHttpConnection(tcpConnection))
	}
}

// FrontDispatch .
func FrontDispatch(httpConnection *utility.HttpConnection) {
	defer httpConnection.Close()

	request, err := httpConnection.ReadRequest(time.Duration(Config.GetInt("externalReadTimeout")) * time.Millisecond)
	if err != nil {
		return
	}
	response := utility.NewHttpResponse()

	// 生成log句柄
	logHandle := utility.NewLogger("")

	pathInfo := strings.Trim(request.PathInfo(), "/")
	parts := strings.Split(pathInfo, "/")
	if len(parts) == 2 && frontPattern.Match([]byte(pathInfo)) {
		methodName := "Do" + strings.Title(parts[0]) + strings.Title(parts[1])
		frontServer := NewFrontServer()
		frontServerValue := reflect.ValueOf(frontServer)
		methodValue := frontServerValue.MethodByName(methodName)
		if methodValue.IsValid() {
			requestValue := reflect.ValueOf(request)
			responseValue := reflect.ValueOf(response)
			logHandleValue := reflect.ValueOf(logHandle)
			methodValue.Call([]reflect.Value{requestValue, responseValue, logHandleValue})
			goto finished
		}
	}
	response.SetCode(404)
	response.Puts("not found")
finished:
	// 每个请求，记录访问日志
	requestLogWrite(request, response, logHandle)

	response.SetHeader("Connection", "close")
	httpConnection.WriteResponse(response, time.Duration(Config.GetInt("externalWriteTimeout"))*time.Millisecond)
}

func requestLogWrite(request *utility.HttpRequest, response *utility.HttpResponse, logHandle *utility.Logger) {
	if request.PathInfo() == "/multi/browse" {
		// 批量接口，不在此记录notice，在接口内部记录
		return
	}

	var logMsg string = ""
	logMsg += "[ cip=" + request.GetRemoteIP()
	logMsg += " intf=" + request.PathInfo()

	gets := request.Gets()
	for k, v := range gets {
		logMsg += " " + k + "=" + v
	}
	posts := request.Posts()
	for k, v := range posts {
		logMsg += " " + k + "=" + v
	}
	logMsg += " ]"
	logMsg += " [ BodyString=" + response.BodyString() + " ]"

	logHandle.Notice(logMsg)
}
