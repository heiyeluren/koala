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

package utility

import (
	"errors"
	"fmt"
	"io"
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

import "github.com/heiyeluren/koala/utility/logger"
import "github.com/heiyeluren/koala/utility/network"

logid := request.Header("WD_REQUEST_ID") //注意:只有问答产品才有WD_REQUEST_ID这个数据，其他服务按照对应id来
log = logger.NewLogger(logid)    //注意: 一个请求只能New一次，logid可以传空字符串，则会内部自己生成logid

log_notice_msg := "[clientip=202.106.51.6 errno=0 errmsg=ok request_time=100ms]"
log.Notice(log_notice_msg)

log_warning_msg := "[clientip=202.106.51.6 errno=101 errmsg=\"client is error\" request_time=45ms]"
log.Warning(log_warning_msg)

*/

// ========================
//
//   外部调用Logger方法
//
// ========================

// Logger .Log 每次请求结构体数据
type Logger struct {
	LogID string
}

// 日志级别类型常量
const (
	LogTypeFatal   = 1
	LogTypeWarning = 2
	LogTypeNotice  = 4
	LogTypeTrace   = 8
	LogTypeDebug   = 16
)

// 日志类型对应信息
const (
	LogTypeFatalStr   = "FATAL"
	LogTypeWarningStr = "WARNING"
	LogTypeNoticeStr  = "NOTICE"
	LogTypeTraceStr   = "TRACE"
	LogTypeDebugStr   = "DEBUG"
)

// GLogTypeMap 日志信息map
var GLogTypeMap = map[int]string{
	LogTypeFatal:   LogTypeFatalStr,
	LogTypeWarning: LogTypeWarningStr,
	LogTypeNotice:  LogTypeNoticeStr,
	LogTypeTrace:   LogTypeTraceStr,
	LogTypeDebug:   LogTypeDebugStr,
}

// ------------------------
//   logger外部调用方法
// ------------------------

// NewLogger . 构造函数
func NewLogger(logid string) *Logger {
	return &Logger{LogID: logid}
}

// Notice .
// 正常请求日志打印调用
// 注意：
// 每个请求(request)只能调用本函数一次，函数里必须携带必须字段: ip, errno, errmsg 等字段，其他kv信息自己组织
// 示例：
// Log_Notice("clientip=192.168.0.1 errno=0 errmsg=ok  key1=valu2 key2=valu2")
func (l *Logger) Notice(logMessage string) {
	l.syncMsg(LogTypeNotice, logMessage)
}

// Trace .
// 函数调用栈trace日志打印调用
func (l *Logger) Trace(logMessage string) {
	l.syncMsg(LogTypeTrace, logMessage)
}

// Debug .
// 函数调用调试debug日志打印调用
func (l *Logger) Debug(logMessage string) {
	l.syncMsg(LogTypeDebug, logMessage)
}

// Fatal .
// 致命错误Fatal日志打印调用
func (l *Logger) Fatal(logMessage string) {
	l.syncMsg(LogTypeFatal, logMessage)
}

// Warning .
// 警告错误warging日志打印调用
func (l *Logger) Warning(logMessage string) {
	l.syncMsg(LogTypeWarning, logMessage)
}

// ------------------------
//   logger内部使用方法
// ------------------------

// syncMsg .
// 写入日志到channel .
func (l *Logger) syncMsg(logType int, logMsg string) error {
	// init request log
	// Log_New()

	// 从配置日志级别log_level判断当前日志是否需要入channel队列
	if (logType & GLogV.LogLevel) != logType {
		return nil
	}

	// G_Log_V := Log_New(G_Log_V)
	if logType <= 0 || logMsg == "" {
		return errors.New("log_type or log_msg param is empty")
	}

	// 拼装消息内容
	logStr := l.padMsg(logType, logMsg)

	// 日志类型
	if _, ok := GLogTypeMap[logType]; !ok {
		return errors.New("log_type is invalid")
	}

	// 设定消息格式
	logMsgData := LogMsgT{
		LogType: logType,
		LogData: logStr,
	}

	// 写消息到channel
	GLogV.LogChan <- logMsgData

	// 判断当前整个channel 的buffer大小是否超过90%的阀值，超过就直接发送刷盘信号
	var threshold float32
	var currChanLen int = len(GLogV.LogChan)
	threshold = float32(currChanLen) / float32(GLogV.LogChanBuffSize)

	if threshold >= 0.9 && !GFlushLogFlag {
		GFlushLock.Lock()
		GFlushLogFlag = true
		GFlushLock.Unlock()

		GLogV.FlushLogChan <- true
		// 打印目前达到阀值了
		if LogIsDebug() {
			LogDebugPrint(fmt.Sprintf("Out threshold!! Current G_Log_V.LogChan: %v; G_Log_V.LogChanBuffSize: %v", currChanLen, GLogV.LogChanBuffSize), nil)
		}
	}

	return nil
}

// padMsg 拼装日志消息
// 说明：主要是按照格式把消息给拼装起来
//
// 日志格式示例：
//  NOTICE: 2013-06-28 18:30:56 koala [logid=1234 filename=yyy.go lineno=29] [clientip=10.5.0.108 errno=0 errmsg="ok"]
//  WARNING: 2013-06-28 18:30:56 koala [logid=1234 filename=yyy.go lineno=29] [clientip=10.5.0.108 errno=404 errmsg="json format invalid"]
func (l *Logger) padMsg(logType int, logMsg string) string {

	var (
		// 日志拼装格式字符串
		logFormatStr string
		logRetStr    string

		// 日志所需字段变量
		logTypeStr  string
		logDateTime string
		logID       string
		logFilename string
		logLineno   int
		logCallFunc string

		// log_clientip string
		// log_errno int
		// log_errmsg string

		// 其他变量
		ok     bool
		fcName uintptr
	)

	// 获取调用的 函数/文件名/行号 等信息
	fcName, logFilename, logLineno, ok = runtime.Caller(3)
	if !ok {
		errors.New("call runtime.Caller() fail")
	}
	logCallFunc = runtime.FuncForPC(fcName).Name()

	// 展现调用文件名最后两段
	// println(log_filename)

	// 判断当前操作系统路径分割符，获取调用文件最后两组路径信息
	osPathSeparator := LogGetOsSeparator(logFilename)
	callPath := strings.Split(logFilename, osPathSeparator)
	if pathLen := len(callPath); pathLen > 2 {
		logFilename = strings.Join(callPath[pathLen-2:], osPathSeparator)
	}

	// 获取当前日期时间 (#吐槽: 不带这么奇葩的调用参数好不啦！难道这天是Go诞生滴日子??!!!#)
	logDateTime = time.Now().Format("2006-01-02 15:04:05")

	// app name
	// log_app_name = "koala"

	// logid读取
	logID = l.getLogID()

	// 日志类型
	if logTypeStr, ok = GLogTypeMap[logType]; !ok {
		errors.New("log_type is invalid")
	}

	// 拼装返回
	logFormatStr = "%s: %s [logid=%s file=%s no=%d call=%s] %s\n"
	logRetStr = fmt.Sprintf(logFormatStr, logTypeStr, logDateTime, logID, logFilename, logLineno, logCallFunc, logMsg)

	// 调试
	// println(log_ret_str)

	return logRetStr
}

// getLogID 获取LogID
// 说明：从客户端request http头里看看是否可以获得logid，http头里可以传递一个：WD_REQUEST_ID
// 如果没有传递，则自己生成唯一logid
func (l *Logger) getLogID() string {
	// 获取request http头中的logid字段
	if l.LogID != "" {
		return l.LogID
	}
	return l.genLogID()
}

// genLogID 生成当前请求的Log ID
// 策略：主要是保证唯一logid，采用当前纳秒级时间+随机数生成
func (l *Logger) genLogID() string {
	// 获取当前时间
	microTime := time.Now().UnixNano()
	// 生成随机数
	randNum := rand.New(rand.NewSource(microTime)).Intn(100000)
	// 生成logid：把纳秒时间+随机数生成 (注意：int64的转string使用 FormatInt，int型使用Itoa就行了)
	// logid := fmt.Sprintf("%d%d", microTime, randNum)
	return strconv.FormatInt(microTime, 10) + strconv.Itoa(randNum)
}

// ========================
//
//   内部协程Run函数
//
// ========================

// LogMsgT 单条日志结构
type LogMsgT struct {
	LogType int
	LogData string
}

// LogT Log主chan队列配置
type LogT struct {

	// ------------------
	//  Channel数据
	// ------------------

	// 日志接收channel队列
	LogChan chan LogMsgT

	// 是否马上日志刷盘: true or false，如果为true，则马上日志刷盘 (本chan暂时没有使用)
	FlushLogChan chan bool

	// ------------------
	// 配置相关数据
	// ------------------

	// 所有日志文件位置
	LogFilePath map[int]string

	// 日志文件位置 (例：/var/log/koala.log 和 /var/log/koala.log.wf)
	LogNoticeFilePath string
	LogErrorFilePath  string

	// 写入日志切割周期（1天:day、1小时:hour、15分钟：Fifteen、10分钟：Ten）
	LogCronTime string

	// 日志chan队列的buffer长度，建议不要少于1024，不多于102400，最长：2147483648
	LogChanBuffSize int

	// 按照间隔时间日志刷盘的日志的间隔时间，单位：秒，建议1~5秒，不超过256
	LogFlushTimer int

	// ------------------
	// 运行时相关数据
	// ------------------

	// 去重的日志文件名和fd (实际需需要物理写入文件名和句柄)
	MergeLogFile map[string]string
	MergeLogFd   map[string]*os.File

	// 上游配置的map数据(必须包含所有所需项)
	RunConfigMap map[string]string

	// 是否开启日志库调试模式
	LogDebugOpen bool

	// 日志打印的级别（需要打印那些日志）
	LogLevel int

	// 日志文件的存在时间, 单位:天
	LogLifeTime int
}

/**
 * 配置项相关常量&变量
 */
const (
	LogConfNoticeFilePath  = "log_notice_file_path"
	LogConfDebugFilePath   = "log_debug_file_path"
	LogConfTraceFilePath   = "log_trace_file_path"
	LogConfFatalFilePath   = "log_fatal_file_path"
	LogConfWarningFilePath = "log_warning_file_path"

	LogConfCronTime     = "log_cron_time"
	LogConfChanBuffSize = "log_channel_buff_size"
	LogConfFlushTimer   = "log_flush_timer"
	LogConfDebugOpen    = "log_debug_open"
	LogConfLevel        = "log_level"
	LogConfFileLifeTime = "log_file_life_time"
)

// 配置选项值类型(字符串或数字)
const (
	LogConfTypeStr = 1
	LogConfTypeNum = 2
)

// GConfItemMap 配置项map全局变量 (定义一个选项输入的值是字符串还是数字)
var GConfItemMap = map[string]int{
	LogConfNoticeFilePath:  LogConfTypeStr,
	LogConfDebugFilePath:   LogConfTypeStr,
	LogConfTraceFilePath:   LogConfTypeStr,
	LogConfFatalFilePath:   LogConfTypeStr,
	LogConfWarningFilePath: LogConfTypeStr,

	LogConfCronTime:     LogConfTypeStr,
	LogConfChanBuffSize: LogConfTypeNum,
	LogConfFlushTimer:   LogConfTypeNum,
	LogConfDebugOpen:    LogConfTypeNum,
	LogConfLevel:        LogConfTypeNum,
	LogConfFileLifeTime: LogConfTypeNum,
}

// GConfFileToTypeMap 日志文件名与日志类型的映射
var GConfFileToTypeMap = map[string]int{
	LogConfNoticeFilePath:  LogTypeNotice,
	LogConfDebugFilePath:   LogTypeDebug,
	LogConfTraceFilePath:   LogTypeTrace,
	LogConfFatalFilePath:   LogTypeFatal,
	LogConfWarningFilePath: LogTypeWarning,
}

// GLogV .日志全局变量
var GLogV *LogT

// GOnceV 全局once
var GOnceV sync.Once

// GFlushLogFlag 目前是否已经写入刷盘操作channel（保证全局只能写入一次，防止多协程操作阻塞）
var GFlushLogFlag bool = false

// GFlushLock 控制 GFlushLogFlag 的全局锁
var GFlushLock *sync.Mutex = &sync.Mutex{}

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
go LogRun(m)
*/

// LogRun . 注意： 需要传递进来的配置是有要求的，必须是包含这些配置选项，否则会报错
func LogRun(RunConfigMap map[string]string) {
	// 初始化全局变量
	if GLogV == nil {
		GLogV = new(LogT)
	}

	// 设置配置map数据
	GLogV.RunConfigMap = RunConfigMap

	// 调用初始化操作，全局只运行一次
	GOnceV.Do(LogInit)

	// 启动log文件清理协程，定期删除过期的log文件
	go LogfileCleanup(int64(GLogV.LogLifeTime * 3600 * 24))

	// 永远循环等待channel的日志数据
	var logMsg LogMsgT
	// var num int64
	for {
		// 监控是否有可以日志可以存取
		select {
		case logMsg = <-GLogV.LogChan:
			LogWriteFile(logMsg)
			// if Log_Is_Debug() {
			//    Log_Debug_Print("G_Log_V.LogChan Length:", len(G_Log_V.LogChan))
			// }
		default:
			// breakLogChan长度
			// println("In Default ", num)
			// 打印目前G_Log_V的数据
			// if Log_Is_Debug() {
			//    Log_Debug_Print("G_Log_V.LogChan Length:", len(G_Log_V.LogChan))
			// }
			time.Sleep(time.Duration(GLogV.LogFlushTimer) * time.Millisecond)
		}

		// 监控刷盘timer
		// log_timer := time.NewTimer(time.Duration(G_Log_V.LogFlushTimer) * time.Millisecond)
		select {
		// 超过设定时间开始检测刷盘（保证不会频繁写日志操作）
		// case <-log_timer.C:
		//    log_timer.Stop()
		//    break
		// 如果收到刷盘channel的信号则刷盘且全局标志状态为
		case <-GLogV.FlushLogChan:
			GFlushLock.Lock()
			GFlushLogFlag = false
			GFlushLock.Unlock()

			// log_timer.Stop()
			break
		default:
			break
		}

	}
}

// LogInit . 初始化Log协程相关操作
// 注意： 全局操作, 只能协程初始化的时候调用一次
func LogInit() {
	if GLogV.RunConfigMap == nil {
		errors.New("Log_Init fail: RunConfigMap data is nil")
	}

	// 构建日志文件名和文件句柄map内存
	GLogV.LogFilePath = make(map[int]string, len(GLogTypeMap))

	// 判断各个配置选项是否存在
	for confItemKey := range GConfItemMap {
		if _, ok := GLogV.RunConfigMap[confItemKey]; !ok {
			fmt.Errorf("Log_Init fail: RunConfigMap not include item: %s", confItemKey)
		}
	}

	// 扫描所有配置选项赋值给结构体
	var err error
	var itemValStr string
	var itemValNum int
	for confItemK, confItemV := range GConfItemMap {
		// 对所有配置选项 进行类型转换
		if confItemV == LogConfTypeStr {
			itemValStr = string(GLogV.RunConfigMap[confItemK])
		} else if confItemV == LogConfTypeNum {
			if itemValNum, err = strconv.Atoi(GLogV.RunConfigMap[confItemK]); err != nil {
				fmt.Errorf("log conf read map[%s] fail, map is error", confItemK)
			}
		}
		// 进行各选项赋值
		switch confItemK {
		// 日志文件路径
		case LogConfNoticeFilePath:
			GLogV.LogFilePath[LogTypeNotice] = itemValStr
		case LogConfDebugFilePath:
			GLogV.LogFilePath[LogTypeDebug] = itemValStr
		case LogConfTraceFilePath:
			GLogV.LogFilePath[LogTypeTrace] = itemValStr
		case LogConfFatalFilePath:
			GLogV.LogFilePath[LogTypeFatal] = itemValStr
		case LogConfWarningFilePath:
			GLogV.LogFilePath[LogTypeWarning] = itemValStr

		// 其他配置选项
		case LogConfCronTime:
			GLogV.LogCronTime = itemValStr
		case LogConfChanBuffSize:
			GLogV.LogChanBuffSize = itemValNum
		case LogConfFlushTimer:
			GLogV.LogFlushTimer = itemValNum
		case LogConfDebugOpen:
			if itemValNum == 1 {
				GLogV.LogDebugOpen = true
			} else {
				GLogV.LogDebugOpen = false
			}
		case LogConfLevel:
			GLogV.LogLevel = itemValNum
		case LogConfFileLifeTime:
			GLogV.LogLifeTime = itemValNum
		}
	}

	// 设置日志channel buffer
	if GLogV.LogChanBuffSize <= 0 {
		GLogV.LogChanBuffSize = 1024
	}
	GLogV.LogChan = make(chan LogMsgT, GLogV.LogChanBuffSize)

	// 初始化唯一的日志文件名和fd
	GLogV.MergeLogFile = make(map[string]string, len(GLogTypeMap))
	GLogV.MergeLogFd = make(map[string]*os.File, len(GLogTypeMap))
	for _, logFilePath := range GLogV.LogFilePath {
		GLogV.MergeLogFile[logFilePath] = ""
		GLogV.MergeLogFd[logFilePath] = nil
	}

	// 打印目前G_Log_V的数据
	if LogIsDebug() {
		LogDebugPrint("G_Log_V data:", GLogV)
	}

	// 设置清理时间不可为0
	if GLogV.LogLifeTime <= 0 {
		GLogV.LogLifeTime = 7 // 默认7天
	}

}

// LogWriteFile .
// 写日志操作
func LogWriteFile(logMsg LogMsgT) {
	// 读取多少行开始写日志
	// var max_line_num int

	// 临时变量
	var (
		// 动态生成需要最终输出的日志map
		logMap map[string][]string
		// 读取单条的日志消息
		logMsgVar LogMsgT
		// 读取单个配置的日志文件名
		confFileName string

		writeBuf string
		line     string
	)

	// 打开文件
	LogOpenFile()

	// 初始化map数据都为
	logMap = make(map[string][]string, len(GConfFileToTypeMap))
	for confFileName = range GLogV.MergeLogFile {
		logMap[confFileName] = []string{}
	}
	// fmt.Println(log_map)

	// 压入第一条读取的日志(上游select读取的)
	confFileName = GLogV.LogFilePath[logMsg.LogType]
	logMap[confFileName] = []string{logMsg.LogData}
	// fmt.Println(log_map)

	// 读取日志(所有可读的日志都读取，然后按照需要打印的文件压入到不同map数组)
	select {
	case logMsgVar = <-GLogV.LogChan:
		confFileName = GLogV.LogFilePath[logMsgVar.LogType]
		logMap[confFileName] = append(logMap[confFileName], logMsgVar.LogData)
	default:
		break
	}
	// 调试信息
	if LogIsDebug() {
		LogDebugPrint("Log Map:", logMap)
	}

	// 写入所有日志(所有map所有文件的都写)
	for confFileName = range GLogV.MergeLogFile {
		if len(logMap[confFileName]) > 0 {
			writeBuf = ""
			for _, line = range logMap[confFileName] {
				writeBuf += line
			}
			_, _ = GLogV.MergeLogFd[confFileName].WriteString(writeBuf)
			_ = GLogV.MergeLogFd[confFileName].Sync()

			// 调试信息
			if LogIsDebug() {
				LogDebugPrint("Log String:", writeBuf)
			}
		}
	}

}

// LogOpenFile .
//  打开&切割日志文件
func LogOpenFile() error {
	var (
		fileSuffix     string
		err            error
		confFileName   string
		runFileName    string
		newLogFileName string
		newLogFileFd   *os.File
	)

	// 构造日志文件名
	fileSuffix = LogGetFileSuffix(time.Now())

	// 把重复日志文件都归一，然后进行相应日志文件的操作
	for confFileName, runFileName = range GLogV.MergeLogFile {
		newLogFileName = fmt.Sprintf("%s.%s", confFileName, fileSuffix)

		// 如果新旧文件名不同，说明需要切割文件了(第一次运行则是全部初始化文件)
		if newLogFileName != runFileName {
			// 关闭旧日志文件
			if GLogV.MergeLogFd[confFileName] != nil {
				if err = GLogV.MergeLogFd[confFileName].Close(); err != nil {
					fmt.Errorf("close log file %s fail", runFileName)
				}
			}
			// 初始化新日志文件
			GLogV.MergeLogFile[confFileName] = newLogFileName
			GLogV.MergeLogFd[confFileName] = nil

			// 创建&打开新日志文件
			newLogFileFd, err = os.OpenFile(newLogFileName, os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				fmt.Errorf("open log file %s fail", newLogFileName)
			}
			newLogFileFd.Seek(0, io.SeekEnd)

			// 把处理的相应的结果进行赋值
			GLogV.MergeLogFile[confFileName] = newLogFileName
			GLogV.MergeLogFd[confFileName] = newLogFileFd
		}
	}

	// 调试
	// fmt.Println(G_Log_V)

	return nil
}

// LogGetFileSuffix .
// 获取日志文件的切割时间
// 说明：
//  目前主要支持三种粒度的设置，基本这些粒度足够我们使用了
// 1天:day; 1小时:hour; 10分钟:ten
func LogGetFileSuffix(now time.Time) string {
	var fileSuffix string
	// now := time.Now()

	switch GLogV.LogCronTime {

	// 按照天切割日志
	case "day":
		fileSuffix = now.Format("20060102")

	// 按照小时切割日志
	case "hour":
		fileSuffix = now.Format("20060102_15")

	// 按照10分钟切割日志
	case "ten":
		fileSuffix = fmt.Sprintf("%s%d0", now.Format("20060102_15"), int(now.Minute()/10))

	// 缺省按照小时
	default:
		fileSuffix = now.Format("20060102_15")
	}

	return fileSuffix
}

// LogIsDebug .
// 获取目前是否是Debug模式
func LogIsDebug() bool {
	return GLogV.LogDebugOpen
}

// LogDebugPrint .
// 日志打印输出到终端函数
func LogDebugPrint(msg string, v interface{}) {

	// 获取调用的 函数/文件名/行号 等信息
	fcName, logFilename, logLineno, ok := runtime.Caller(1)
	if !ok {
		errors.New("call runtime.Caller() fail")
	}
	logCallFunc := runtime.FuncForPC(fcName).Name()

	osPathSeparator := LogGetOsSeparator(logFilename)
	callPath := strings.Split(logFilename, osPathSeparator)
	if pathLen := len(callPath); pathLen > 2 {
		logFilename = strings.Join(callPath[pathLen-2:], osPathSeparator)
	}

	fmt.Println("\n=======================Log Debug Info Start=======================")
	fmt.Println("[ call=", logCallFunc, "file=", logFilename, "no=", logLineno, "]")
	if msg != "" {
		fmt.Println(msg)
	}
	fmt.Println(v)
	fmt.Println("=======================Log Debug Info End=======================\n")
}

// LogGetOsSeparator .
// 获取当前操作系统的路径切割符
// 说明: 主要为了解决 os.PathSeparator有些时候无法满足要求的问题
func LogGetOsSeparator(pathName string) string {
	// 判断当前操作系统路径分割符
	var osPathSeparator = "/"
	if strings.ContainsAny(pathName, "\\") {
		osPathSeparator = "\\"
	}
	return osPathSeparator
}

// LogfileCleanup .
// 对notice日志，进行定期清理，执行周期等同于“日志切割周期”
func LogfileCleanup(fileLifetime int64) {

	// println("clean up goroutine start!")

	// 5秒后再启动“清理”循环，错开启动初期的不稳定时段
	time.Sleep(time.Duration(5) * time.Second)

	var (
		// 清理周期，秒
		cycleTime int64
		// log文件保存周期，秒；  30天
		// file_lifetime int64 = 3600 * 24 * 30
	)

	// 清理周期，设置为 “日志切割时间 ”
	switch GLogV.LogCronTime {
	case "day":
		cycleTime = 3600 * 24
	case "hour":
		cycleTime = 3600
	case "ten":
		cycleTime = 600
	default:
		cycleTime = 3600
	}

	// 目前，仅针对notice日志（所在log文件）进行删除操作
	confFileName := GLogV.LogFilePath[LogTypeNotice]

	// 删除log文件无限循环
	for {
		var cleanupTime time.Time = time.Unix(time.Now().Unix()-fileLifetime, 0)

		// 计算出，待删除log文件名
		fileSuffix := LogGetFileSuffix(cleanupTime)

		// println(log_file_name)

		// 删除log文件
		// if err := os.Remove(log_file_name); err != nil {
		//     println(err.Error())
		// }

		os.Remove(fmt.Sprintf("%s.%s", confFileName, fileSuffix))
		// 等待下一个清理周期，sleep
		time.Sleep(time.Duration(cycleTime) * time.Second)
	}
}
