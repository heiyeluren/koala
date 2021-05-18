package main

/*
import (
    "fmt"
    "io/ioutil"
    "os"
    "utility/logger"
    "utility/network"
)

暂时删除
func (this *FrontServer) DoRuleRewrite(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    ext_stream := request.Pstr("rule_stream")
    if ext_stream == "" {
        response.Puts(`{"errno": -1, "errmsg": "no rule stream"}`)
        response.SetCode(400)
        return
    }
    err := PolicyInterpreter(ext_stream)
    if err != nil {
        logHandle.Warning("[errmsg=" + err.Error() + "]")
        response.Puts(`{"errno": -2, "errmsg": "rule format error! ;` + err.Error() + `"}`)
        response.SetCode(400)
        return
    }
    err = ioutil.WriteFile(Config.Get("rule_file"), []byte(ext_stream), os.FileMode.Perm(777))
    if err != nil {
        logHandle.Warning("[errmsg=" + err.Error() + "]")
    }
    response.Puts(`{"errno": 0, "errmsg": "rule rewrite success!"}`)
    response.SetCode(200)
}
*/

/*
func (this *FrontServer) DoDumpCounter(request *network.HttpRequest, response *network.HttpResponse, logHandle *logger.Logger) {
    retString := ""
    rule_no := int32(request.Gint("rule_no"))
    counters := GetCurrentCounters()
    if rule_no != 0 {
        if _, OK := counters[rule_no]; !OK {
            response.Puts("rule no found or not used!")
            response.SetCode(400)
            return
        }
        retString += fmt.Sprintf("  rule_no:%d  allow:%d  deny:%d ", rule_no, counters[rule_no].allow, counters[rule_no].deny)
    } else {
        for k, v := range counters {
            retString += fmt.Sprintf("  rule_no:%d  allow:%d  deny:%d  <br />", k, v.allow, v.deny)
        }
    }

    response.Puts(retString)
    response.SetCode(200)
}
*/
