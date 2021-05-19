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

package main

import (
    "reflect"
    "regexp"
    "strings"
    "time"
    "utility/logger"
    "utility/network"
)

var frontPattern = regexp.MustCompile("^[a-z][0-9a-z_]*/[a-z][0-9a-z_]*$")

type FrontServer struct{}

func NewFrontServer() *FrontServer {
    return &FrontServer{}
}

func FrontListen() {
    tcpListener, err := network.TcpListen(Config.Get("listen"))
    if err != nil {
        panic(err.Error())
    }
    for {
        tcpConnection, err := tcpListener.Accept()
        if err != nil {
            continue
        }
        go FrontDispatch(network.NewHttpConnection(tcpConnection))
    }
}

func FrontDispatch(httpConnection *network.HttpConnection) {
    defer httpConnection.Close()

    request, err := httpConnection.ReadRequest(time.Duration(Config.GetInt("externalReadTimeout")) * time.Millisecond)
    if err != nil {
        return
    }
    response := network.NewHttpResponse()

    // 生成log句柄
    logHandle := logger.NewLogger("")

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

func requestLogWrite(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
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
