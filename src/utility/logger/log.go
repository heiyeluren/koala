/**
 * @file: log.go
 * @package: main
 * @author: heiyeluren
 * @desc: Log operate file
 * @date: 2013/6/24
 * @history:
 *     2013/6/24 created file
 *     2013/7/1  add logid function
 *     2013/7/2  update code structure
 *     2013/7/4  refactor all code
 *     2013/7/10 add log_level operate
 */

package logger

import (
    "errors"
    "fmt"
    "math/rand"
    "os"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "time"
)

/*

配置文件格式：
=================================================
#日志文件位置 (例：/var/log/koala.log)
log_notice_file_path    = log/koala.log
log_debug_file_path = log/koala.log
log_trace_file_path = log/koala.log
log_fatal_file_path = log/koala.log.wf
log_warning_file_path   = log/koala.log.wf

#日志文件切割周期（1天:day; 1小时:hour; 10分钟:ten）
log_cron_time = day

#日志chan队列的buffer长度，建议不要少于1024，#不建议多于102400，最长：2147483648
log_chan_buff_size = 20480

#日志刷盘的间隔时间，单位:毫秒，建议500~5000毫秒(0.5s-5s)，建议不超过30秒
log_flush_timer = 1000
=================================================

代码调用示例：

import "utility/logger"
import "utility/network"

logid := request.Header("WD_REQUEST_ID") //注意:只有问答产品才有WD_REQUEST_ID这个数据，其他服务按照对应id来
log = logger.NewLogger(logid)    //注意: 一个请求只能New一次，logid可以传空字符串，则会内部自己生成logid

log_notice_msg := "[clientip=202.106.51.6 errno=0 errmsg=ok request_time=100ms]"
log.Notice(log_notice_msg)

log_warning_msg := "[clientip=202.106.51.6 errno=101 errmsg=\"client is error\" request_time=45ms]"
log.Warning(log_warning_msg)

*/

//========================
//
//   外部调用Logger方法
//
//========================

/**
 * Log 每次请求结构体数据
 */
type Logger struct {
    Logid string
}

//日志级别类型常量
const (
    LOG_TYPE_FATAL   = 1
    LOG_TYPE_WARNING = 2
    LOG_TYPE_NOTICE  = 4
    LOG_TYPE_TRACE   = 8
    LOG_TYPE_DEBUG   = 16
)

//日志类型对应信息
const (
    LOG_TYPE_FATAL_STR   = "FATAL"
    LOG_TYPE_WARNING_STR = "WARNING"
    LOG_TYPE_NOTICE_STR  = "NOTICE"
    LOG_TYPE_TRACE_STR   = "TRACE"
    LOG_TYPE_DEBUG_STR   = "DEBUG"
)

//日志信息map
var G_Log_Type_Map map[int]string = map[int]string{
    LOG_TYPE_FATAL:   LOG_TYPE_FATAL_STR,
    LOG_TYPE_WARNING: LOG_TYPE_WARNING_STR,
    LOG_TYPE_NOTICE:  LOG_TYPE_NOTICE_STR,
    LOG_TYPE_TRACE:   LOG_TYPE_TRACE_STR,
    LOG_TYPE_DEBUG:   LOG_TYPE_DEBUG_STR,
}

//------------------------
//   logger外部调用方法
//------------------------

/**
 * 构造函数
 *
 */
func NewLogger(logid string) *Logger {
    return &Logger{Logid: logid}
}

/**
 * 正常请求日志打印调用
 *
 * 注意：
 * 每个请求(request)只能调用本函数一次，函数里必须携带必须字段: ip, errno, errmsg 等字段，其他kv信息自己组织
 *
 * 示例：
 * Log_Notice("clientip=192.168.0.1 errno=0 errmsg=ok  key1=valu2 key2=valu2")
 *
 */
func (l *Logger) Notice(log_messgae string) {
    l.sync_msg(LOG_TYPE_NOTICE, log_messgae)
}

/**
 * 函数调用栈trace日志打印调用
 *
 */
func (l *Logger) Trace(log_messgae string) {
    l.sync_msg(LOG_TYPE_TRACE, log_messgae)
}

/**
 * 函数调用调试debug日志打印调用
 *
 */
func (l *Logger) Debug(log_messgae string) {
    l.sync_msg(LOG_TYPE_DEBUG, log_messgae)
}

/**
 * 致命错误Fatal日志打印调用
 *
 */
func (l *Logger) Fatal(log_messgae string) {
    l.sync_msg(LOG_TYPE_FATAL, log_messgae)
}

/**
 * 警告错误warging日志打印调用
 *
 */
func (l *Logger) Warning(log_messgae string) {
    l.sync_msg(LOG_TYPE_WARNING, log_messgae)
}

//------------------------
//   logger内部使用方法
//------------------------

/**
 * 写入日志到channel
 *
 */
func (l *Logger) sync_msg(log_type int, log_msg string) error {
    //init request log
    //Log_New()

    //从配置日志级别log_level判断当前日志是否需要入channel队列
    if (log_type & G_Log_V.LogLevel) != log_type {
        return nil
    }

    //G_Log_V := Log_New(G_Log_V)
    if log_type <= 0 || log_msg == "" {
        errors.New("log_type or log_msg param is empty")
    }

    //拼装消息内容
    log_str := l.pad_msg(log_type, log_msg)

    //日志类型
    if _, ok := G_Log_Type_Map[log_type]; !ok {
        errors.New("log_type is invalid")
    }

    //设定消息格式
    log_msg_data := Log_Msg_T{
        LogType: log_type,
        LogData: log_str,
    }

    //写消息到channel
    G_Log_V.LogChan <- log_msg_data

    //判断当前整个channel 的buffer大小是否超过90%的阀值，超过就直接发送刷盘信号
    var threshold float32
    var curr_chan_len int = len(G_Log_V.LogChan)
    threshold = float32(curr_chan_len) / float32(G_Log_V.LogChanBuffSize)

    if threshold >= 0.9 && G_Flush_Log_Flag != true {
        G_Flush_Lock.Lock()
        G_Flush_Log_Flag = true
        G_Flush_Lock.Unlock()

        G_Log_V.FlushLogChan <- true
        //打印目前达到阀值了
        if Log_Is_Debug() {
            Log_Debug_Print(fmt.Sprintf("Out threshold!! Current G_Log_V.LogChan: %v; G_Log_V.LogChanBuffSize: %v", curr_chan_len, G_Log_V.LogChanBuffSize), nil)
        }
    }

    return nil
}

/**
 * 拼装日志消息
 *
 * 说明：
 * 主要是按照格式把消息给拼装起来
 *
 * 日志格式示例：
 *  NOTICE: 2013-06-28 18:30:56 koala [logid=1234 filename=yyy.go lineno=29] [clientip=10.5.0.108 errno=0 errmsg="ok"]
 *  WARNING: 2013-06-28 18:30:56 koala [logid=1234 filename=yyy.go lineno=29] [clientip=10.5.0.108 errno=404 errmsg="json format invalid"]
 */
func (l *Logger) pad_msg(log_type int, log_msg string) string {

    var (
        //日志拼装格式字符串
        log_format_str string
        log_ret_str    string

        //日志所需字段变量
        log_type_str  string
        log_date_time string
        log_id        string
        log_filename  string
        log_lineno    int
        log_callfunc  string

        //log_clientip string
        //log_errno int
        //log_errmsg string

        //其他变量
        ok     bool
        fcName uintptr
    )

    //获取调用的 函数/文件名/行号 等信息
    fcName, log_filename, log_lineno, ok = runtime.Caller(3)
    if !ok {
        errors.New("call runtime.Caller() fail")
    }
    log_callfunc = runtime.FuncForPC(fcName).Name()

    //展现调用文件名最后两段
    //println(log_filename)

    //判断当前操作系统路径分割符，获取调用文件最后两组路径信息
    os_path_separator := Log_Get_Os_Separator(log_filename)
    call_path := strings.Split(log_filename, os_path_separator)
    if path_len := len(call_path); path_len > 2 {
        log_filename = strings.Join(call_path[path_len-2:], os_path_separator)
    }

    //获取当前日期时间 (#吐槽: 不带这么奇葩的调用参数好不啦！难道这天是Go诞生滴日子??!!!#)
    log_date_time = time.Now().Format("2006-01-02 15:04:05")

    //app name
    //log_app_name = "koala"

    //logid读取
    log_id = l.get_logid()

    //日志类型
    if log_type_str, ok = G_Log_Type_Map[log_type]; !ok {
        errors.New("log_type is invalid")
    }

    //拼装返回
    log_format_str = "%s: %s [logid=%s file=%s no=%d call=%s] %s\n"
    log_ret_str = fmt.Sprintf(log_format_str, log_type_str, log_date_time, log_id, log_filename, log_lineno, log_callfunc, log_msg)

    //调试
    //println(log_ret_str)

    return log_ret_str
}

/**
 * 获取LogID
 *
 * 说明：
 * 从客户端request http头里看看是否可以获得logid，http头里可以传递一个：WD_REQUEST_ID
 * 如果没有传递，则自己生成唯一logid
 */
func (l *Logger) get_logid() string {
    //获取request http头中的logid字段
    if l.Logid != "" {
        return l.Logid
    }
    return l.gen_logid()
}

/**
 * 生成当前请求的Log ID
 *
 * 策略：
 * 主要是保证唯一logid，采用当前纳秒级时间+随机数生成
 */
func (l *Logger) gen_logid() string {
    //获取当前时间
    microtime := time.Now().UnixNano()

    //生成随机数
    r := rand.New(rand.NewSource(microtime))
    randNum := r.Intn(100000)

    //生成logid：把纳秒时间+随机数生成 (注意：int64的转string使用 FormatInt，int型使用Itoa就行了)
    //logid := fmt.Sprintf("%d%d", microtime, randNum)
    logid := strconv.FormatInt(microtime, 10) + strconv.Itoa(randNum)

    return logid
}

//========================
//
//   内部协程Run函数
//
//========================

/**
 * 单条日志结构
 */
type Log_Msg_T struct {
    LogType int
    LogData string
}

/**
 * Log主chan队列配置
 */
type Log_T struct {

    //------------------
    //  Channel数据
    //------------------

    //日志接收channel队列
    LogChan chan Log_Msg_T

    //是否马上日志刷盘: true or false，如果为true，则马上日志刷盘 (本chan暂时没有使用)
    FlushLogChan chan bool

    //------------------
    // 配置相关数据
    //------------------

    //所有日志文件位置
    LogFilePath map[int]string

    //日志文件位置 (例：/var/log/koala.log 和 /var/log/koala.log.wf)
    LogNoticeFilePath string
    LogErrorFilePath  string

    //写入日志切割周期（1天:day、1小时:hour、15分钟：Fifteen、10分钟：Ten）
    LogCronTime string

    //日志chan队列的buffer长度，建议不要少于1024，不多于102400，最长：2147483648
    LogChanBuffSize int

    //按照间隔时间日志刷盘的日志的间隔时间，单位：秒，建议1~5秒，不超过256
    LogFlushTimer int

    //------------------
    // 运行时相关数据
    //------------------

    //去重的日志文件名和fd (实际需需要物理写入文件名和句柄)
    MergeLogFile map[string]string
    MergeLogFd   map[string]*os.File

    //上游配置的map数据(必须包含所有所需项)
    RunConfigMap map[string]string

    //是否开启日志库调试模式
    LogDebugOpen bool

    //日志打印的级别（需要打印那些日志）
    LogLevel int

    // 日志文件的存在时间, 单位:天
    LogLifeTime int
}

/**
 * 配置项相关常量&变量
 */
const (
    LOG_CONF_NOTICE_FILE_PATH  = "log_notice_file_path"
    LOG_CONF_DEBUG_FILE_PATH   = "log_debug_file_path"
    LOG_CONF_TRACE_FILE_PATH   = "log_trace_file_path"
    LOG_CONF_FATAL_FILE_PATH   = "log_fatal_file_path"
    LOG_CONF_WARNING_FILE_PATH = "log_warning_file_path"

    LOG_CONF_CRON_TIME      = "log_cron_time"
    LOG_CONF_CHAN_BUFFSIZE  = "log_chan_buff_size"
    LOG_CONF_FLUSH_TIMER    = "log_flush_timer"
    LOG_CONF_DEBUG_OPEN     = "log_debug_open"
    LOG_CONF_LEVEL          = "log_level"
    LOG_CONF_FILE_LIFE_TIME = "log_file_ilfe_time"
)

//配置选项值类型(字符串或数字)
const (
    LOG_CONF_TYPE_STR = 1
    LOG_CONF_TYPE_NUM = 2
)

//配置项map全局变量 (定义一个选项输入的值是字符串还是数字)
var G_Conf_Item_Map map[string]int = map[string]int{
    LOG_CONF_NOTICE_FILE_PATH:  LOG_CONF_TYPE_STR,
    LOG_CONF_DEBUG_FILE_PATH:   LOG_CONF_TYPE_STR,
    LOG_CONF_TRACE_FILE_PATH:   LOG_CONF_TYPE_STR,
    LOG_CONF_FATAL_FILE_PATH:   LOG_CONF_TYPE_STR,
    LOG_CONF_WARNING_FILE_PATH: LOG_CONF_TYPE_STR,

    LOG_CONF_CRON_TIME:      LOG_CONF_TYPE_STR,
    LOG_CONF_CHAN_BUFFSIZE:  LOG_CONF_TYPE_NUM,
    LOG_CONF_FLUSH_TIMER:    LOG_CONF_TYPE_NUM,
    LOG_CONF_DEBUG_OPEN:     LOG_CONF_TYPE_NUM,
    LOG_CONF_LEVEL:          LOG_CONF_TYPE_NUM,
    LOG_CONF_FILE_LIFE_TIME: LOG_CONF_TYPE_NUM,
}

//日志文件名与日志类型的映射
var G_Conf_FileToType_Map map[string]int = map[string]int{
    LOG_CONF_NOTICE_FILE_PATH:  LOG_TYPE_NOTICE,
    LOG_CONF_DEBUG_FILE_PATH:   LOG_TYPE_DEBUG,
    LOG_CONF_TRACE_FILE_PATH:   LOG_TYPE_TRACE,
    LOG_CONF_FATAL_FILE_PATH:   LOG_TYPE_FATAL,
    LOG_CONF_WARNING_FILE_PATH: LOG_TYPE_WARNING,
}

//日志全局变量
var G_Log_V *Log_T

//全局once
var G_Once_V sync.Once

//目前是否已经写入刷盘操作channel（保证全局只能写入一次，防止多协程操作阻塞）
var G_Flush_Log_Flag bool = false

//控制 G_Flush_Log_Flag 的全局锁
var G_Flush_Lock *sync.Mutex = &sync.Mutex{}

/**
* 提供给协程调用的入口函数
*
* @param RunConfigMap 是需要传递进来的配置信息key=>val的map数据
*　调用示例：
*
//注意本调用必须在单独协程里运行
m := map[string]string {
    "log_notice_file_path":     "log/koala.log"
    "log_debug_file_path":      "log/koala.log"
    "log_trace_file_path":      "log/koala.log"
    "log_fatal_file_path":      "log/koala.log.wf"
    "log_warning_file_path":    "log/koala.log.wf"
    "log_cron_time":            "day"
    "log_chan_buff_size":       "10240"
    "log_flush_timer":          "1"
}
go Log_Run(m)

* 注意：
* 需要传递进来的配置是有要求的，必须是包含这些配置选项，否则会报错
*
*/
func Log_Run(RunConfigMap map[string]string) {
    //初始化全局变量
    if G_Log_V == nil {
        G_Log_V = new(Log_T)
    }

    //设置配置map数据
    G_Log_V.RunConfigMap = RunConfigMap

    //调用初始化操作，全局只运行一次
    G_Once_V.Do(Log_Init)

    //启动log文件清理协程，定期删除过期的log文件
    go LogFile_CleanUP(int64(G_Log_V.LogLifeTime * 3600 * 24))

    //永远循环等待channel的日志数据
    var log_msg Log_Msg_T
    //var num int64
    for {
        //监控是否有可以日志可以存取
        select {
        case log_msg = <-G_Log_V.LogChan:
            Log_Write_File(log_msg)
            //if Log_Is_Debug() {
            //    Log_Debug_Print("G_Log_V.LogChan Length:", len(G_Log_V.LogChan))
            //}
        default:
            //breakLogChan长度
            //println("In Default ", num)
            //打印目前G_Log_V的数据
            //if Log_Is_Debug() {
            //    Log_Debug_Print("G_Log_V.LogChan Length:", len(G_Log_V.LogChan))
            //}
            time.Sleep(time.Duration(G_Log_V.LogFlushTimer) * time.Millisecond)
        }

        //监控刷盘timer
        //log_timer := time.NewTimer(time.Duration(G_Log_V.LogFlushTimer) * time.Millisecond)
        select {
        //超过设定时间开始检测刷盘（保证不会频繁写日志操作）
        //case <-log_timer.C:
        //    log_timer.Stop()
        //    break
        //如果收到刷盘channel的信号则刷盘且全局标志状态为
        case <-G_Log_V.FlushLogChan:
            G_Flush_Lock.Lock()
            G_Flush_Log_Flag = false
            G_Flush_Lock.Unlock()

            //log_timer.Stop()
            break
        default:
            break
        }

    }
}

/**
 * 初始化Log协程相关操作
 *
 * 注意：
 * 全局操作, 只能协程初始化的时候调用一次
 *
 */
func Log_Init() {
    if G_Log_V.RunConfigMap == nil {
        errors.New("Log_Init fail: RunConfigMap data is nil")
    }

    //构建日志文件名和文件句柄map内存
    G_Log_V.LogFilePath = make(map[int]string, len(G_Log_Type_Map))

    //判断各个配置选项是否存在
    for conf_item_key, _ := range G_Conf_Item_Map {
        if _, ok := G_Log_V.RunConfigMap[conf_item_key]; !ok {
            errors.New(fmt.Sprintf("Log_Init fail: RunConfigMap not include item: %s", conf_item_key))
        }
    }

    //扫描所有配置选项赋值给结构体
    var err error
    var item_val_str string
    var item_val_num int
    for conf_item_k, conf_item_v := range G_Conf_Item_Map {
        //对所有配置选项 进行类型转换
        if conf_item_v == LOG_CONF_TYPE_STR {
            item_val_str = string(G_Log_V.RunConfigMap[conf_item_k])
        } else if conf_item_v == LOG_CONF_TYPE_NUM {
            if item_val_num, err = strconv.Atoi(G_Log_V.RunConfigMap[conf_item_k]); err != nil {
                errors.New(fmt.Sprintf("Log conf read map[%s] fail, map is error", conf_item_k))
            }
        }
        //进行各选项赋值
        switch conf_item_k {
        //日志文件路径
        case LOG_CONF_NOTICE_FILE_PATH:
            G_Log_V.LogFilePath[LOG_TYPE_NOTICE] = item_val_str
        case LOG_CONF_DEBUG_FILE_PATH:
            G_Log_V.LogFilePath[LOG_TYPE_DEBUG] = item_val_str
        case LOG_CONF_TRACE_FILE_PATH:
            G_Log_V.LogFilePath[LOG_TYPE_TRACE] = item_val_str
        case LOG_CONF_FATAL_FILE_PATH:
            G_Log_V.LogFilePath[LOG_TYPE_FATAL] = item_val_str
        case LOG_CONF_WARNING_FILE_PATH:
            G_Log_V.LogFilePath[LOG_TYPE_WARNING] = item_val_str

        //其他配置选项
        case LOG_CONF_CRON_TIME:
            G_Log_V.LogCronTime = item_val_str
        case LOG_CONF_CHAN_BUFFSIZE:
            G_Log_V.LogChanBuffSize = item_val_num
        case LOG_CONF_FLUSH_TIMER:
            G_Log_V.LogFlushTimer = item_val_num
        case LOG_CONF_DEBUG_OPEN:
            if item_val_num == 1 {
                G_Log_V.LogDebugOpen = true
            } else {
                G_Log_V.LogDebugOpen = false
            }
        case LOG_CONF_LEVEL:
            G_Log_V.LogLevel = item_val_num
        case LOG_CONF_FILE_LIFE_TIME:
            G_Log_V.LogLifeTime = item_val_num
        }
    }

    //设置日志channel buffer
    if G_Log_V.LogChanBuffSize <= 0 {
        G_Log_V.LogChanBuffSize = 1024
    }
    G_Log_V.LogChan = make(chan Log_Msg_T, G_Log_V.LogChanBuffSize)

    //初始化唯一的日志文件名和fd
    G_Log_V.MergeLogFile = make(map[string]string, len(G_Log_Type_Map))
    G_Log_V.MergeLogFd = make(map[string]*os.File, len(G_Log_Type_Map))
    for _, log_file_path := range G_Log_V.LogFilePath {
        G_Log_V.MergeLogFile[log_file_path] = ""
        G_Log_V.MergeLogFd[log_file_path] = nil
    }

    //打印目前G_Log_V的数据
    if Log_Is_Debug() {
        Log_Debug_Print("G_Log_V data:", G_Log_V)
    }

    // 设置清理时间不可为0
    if G_Log_V.LogLifeTime <= 0 {
        G_Log_V.LogLifeTime = 7 // 默认7天
    }

}

/**
 * 写日志操作
 *
 */
func Log_Write_File(log_msg Log_Msg_T) {
    //读取多少行开始写日志
    //var max_line_num int

    //临时变量
    var (
        //动态生成需要最终输出的日志map
        log_map map[string][]string
        //读取单条的日志消息
        log_msg_var Log_Msg_T
        //读取单个配置的日志文件名
        conf_file_name string

        write_buf string
        line      string
    )

    //打开文件
    Log_Open_File()

    //初始化map数据都为
    log_map = make(map[string][]string, len(G_Conf_FileToType_Map))
    for conf_file_name, _ = range G_Log_V.MergeLogFile {
        log_map[conf_file_name] = []string{}
    }
    //fmt.Println(log_map)

    //压入第一条读取的日志(上游select读取的)
    conf_file_name = G_Log_V.LogFilePath[log_msg.LogType]
    log_map[conf_file_name] = []string{log_msg.LogData}
    //fmt.Println(log_map)

    //读取日志(所有可读的日志都读取，然后按照需要打印的文件压入到不同map数组)
    select {
    case log_msg_var = <-G_Log_V.LogChan:
        conf_file_name = G_Log_V.LogFilePath[log_msg_var.LogType]
        log_map[conf_file_name] = append(log_map[conf_file_name], log_msg_var.LogData)
    default:
        break
    }
    //调试信息
    if Log_Is_Debug() {
        Log_Debug_Print("Log Map:", log_map)
    }

    //写入所有日志(所有map所有文件的都写)
    for conf_file_name, _ = range G_Log_V.MergeLogFile {
        if len(log_map[conf_file_name]) > 0 {
            write_buf, line = "", ""
            for _, line = range log_map[conf_file_name] {
                write_buf += line
            }
            _, _ = G_Log_V.MergeLogFd[conf_file_name].WriteString(write_buf)
            _ = G_Log_V.MergeLogFd[conf_file_name].Sync()

            //调试信息
            if Log_Is_Debug() {
                Log_Debug_Print("Log String:", write_buf)
            }
        }
    }

}

/**
 * 打开&切割日志文件
 *
 */
func Log_Open_File() error {
    var (
        file_suffix       string
        err               error
        conf_file_name    string
        run_file_name     string
        new_log_file_name string
        new_log_file_fd   *os.File
    )

    //构造日志文件名
    file_suffix = Log_Get_File_Suffix(time.Now())

    //把重复日志文件都归一，然后进行相应日志文件的操作
    for conf_file_name, run_file_name = range G_Log_V.MergeLogFile {
        new_log_file_name = fmt.Sprintf("%s.%s", conf_file_name, file_suffix)

        //如果新旧文件名不同，说明需要切割文件了(第一次运行则是全部初始化文件)
        if new_log_file_name != run_file_name {
            //关闭旧日志文件
            if G_Log_V.MergeLogFd[conf_file_name] != nil {
                if err = G_Log_V.MergeLogFd[conf_file_name].Close(); err != nil {
                    errors.New(fmt.Sprintf("Close log file %s fail", run_file_name))
                }
            }
            //初始化新日志文件
            G_Log_V.MergeLogFile[conf_file_name] = new_log_file_name
            G_Log_V.MergeLogFd[conf_file_name] = nil

            //创建&打开新日志文件
            new_log_file_fd, err = os.OpenFile(new_log_file_name, os.O_WRONLY|os.O_CREATE, 0644)
            if err != nil {
                errors.New(fmt.Sprintf("Open log file %s fail", new_log_file_name))
            }
            new_log_file_fd.Seek(0, os.SEEK_END)

            //把处理的相应的结果进行赋值
            G_Log_V.MergeLogFile[conf_file_name] = new_log_file_name
            G_Log_V.MergeLogFd[conf_file_name] = new_log_file_fd
        }
    }

    //调试
    //fmt.Println(G_Log_V)

    return nil
}

/**
 * 获取日志文件的切割时间
 *
 * 说明：
 *  目前主要支持三种粒度的设置，基本这些粒度足够我们使用了
 *  1天:day; 1小时:hour; 10分钟:ten
 */
func Log_Get_File_Suffix(now time.Time) string {
    var file_suffix string
    //now := time.Now()

    switch G_Log_V.LogCronTime {

    //按照天切割日志
    case "day":
        file_suffix = now.Format("20060102")

    //按照小时切割日志
    case "hour":
        file_suffix = now.Format("20060102_15")

    //按照10分钟切割日志
    case "ten":
        file_suffix = fmt.Sprintf("%s%d0", now.Format("20060102_15"), int(now.Minute()/10))

    //缺省按照小时
    default:
        file_suffix = now.Format("20060102_15")
    }

    return file_suffix
}

/**
 * 获取目前是否是Debug模式
 *
 */
func Log_Is_Debug() bool {
    if G_Log_V.LogDebugOpen {
        return true
    }
    return false
}

/**
 * 日志打印输出到终端函数
 *
 */
func Log_Debug_Print(msg string, v interface{}) {

    //获取调用的 函数/文件名/行号 等信息
    fcName, log_filename, log_lineno, ok := runtime.Caller(1)
    if !ok {
        errors.New("call runtime.Caller() fail")
    }
    log_callfunc := runtime.FuncForPC(fcName).Name()

    os_path_separator := Log_Get_Os_Separator(log_filename)
    call_path := strings.Split(log_filename, os_path_separator)
    if path_len := len(call_path); path_len > 2 {
        log_filename = strings.Join(call_path[path_len-2:], os_path_separator)
    }

    fmt.Println("\n=======================Log Debug Info Start=======================")
    fmt.Println("[ call=", log_callfunc, "file=", log_filename, "no=", log_lineno, "]")
    if msg != "" {
        fmt.Println(msg)
    }
    fmt.Println(v)
    fmt.Println("=======================Log Debug Info End=======================\n")
}

/**
 * 获取当前操作系统的路径切割符
 *
 * 说明: 主要为了解决 os.PathSeparator有些时候无法满足要求的问题
 *
 */
func Log_Get_Os_Separator(path_name string) string {
    //判断当前操作系统路径分割符
    var os_path_separator = "/"
    if strings.ContainsAny(path_name, "\\") {
        os_path_separator = "\\"
    }
    return os_path_separator
}

/**
 * 对notice日志，进行定期清理，执行周期等同于“日志切割周期”
 */
func LogFile_CleanUP(file_lifetime int64) {

    // println("clean up gorouting start!")

    // 5秒后再启动“清理”循环，错开启动初期的不稳定时段
    time.Sleep(time.Duration(5) * time.Second)

    var (
        // 清理周期，秒
        cycle_time int64
        // log文件保存周期，秒；  30天
        // file_lifetime int64 = 3600 * 24 * 30
    )

    // 清理周期，设置为 “日志切割时间 ”
    switch G_Log_V.LogCronTime {
    case "day":
        cycle_time = 3600 * 24
    case "hour":
        cycle_time = 3600
    case "ten":
        cycle_time = 600
    default:
        cycle_time = 3600
    }

    // 目前，仅针对notice日志（所在log文件）进行删除操作
    conf_file_name := G_Log_V.LogFilePath[LOG_TYPE_NOTICE]

    // 删除log文件无限循环
    for {
        var cleanUp_time time.Time = time.Unix(time.Now().Unix()-file_lifetime, 0)

        // 计算出，待删除log文件名
        file_suffix := Log_Get_File_Suffix(cleanUp_time)
        log_file_name := fmt.Sprintf("%s.%s", conf_file_name, file_suffix)

        // println(log_file_name)

        // 删除log文件
        // if err := os.Remove(log_file_name); err != nil {
        //     println(err.Error())
        // }

        os.Remove(log_file_name)
        // 等待下一个清理周期，sleep
        time.Sleep(time.Duration(cycle_time) * time.Second)
    }
}
