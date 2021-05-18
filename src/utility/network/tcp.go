package network

import (
    "encoding/gob"
    "errors"
    "net"
    "time"
)

// 监听一个 tcp 地址
func TcpListen(address string) (*TcpListener, error) {
    tcpAddr, err := net.ResolveTCPAddr("tcp", address)
    if err != nil {
        return nil, err
    }
    tcpListener, err := net.ListenTCP("tcp", tcpAddr)
    if err != nil {
        return nil, err
    }
    return newTcpListener(tcpListener), nil
}

// tcp 监听器
type TcpListener struct {
    *net.TCPListener
}

func newTcpListener(tcpListener *net.TCPListener) *TcpListener {
    return &TcpListener{TCPListener: tcpListener}
}

func (this *TcpListener) Accept() (*TcpConnection, error) {
    tcpConn, err := this.TCPListener.AcceptTCP()
    if err != nil {
        return nil, err
    }
    return newTcpConnection(tcpConn), nil
}

// 连接一个 tcp 地址
func TcpConnect(localAddress string, remoteAddress string, timeout time.Duration) (*TcpConnection, error) {
    // @todo: 本 api 调用不绑定 localAddress，可能会影响上层应用程序
    conn, err := net.DialTimeout("tcp", remoteAddress, timeout)
    if err != nil {
        return nil, err
    }
    tcpConn := conn.(*net.TCPConn)
    return newTcpConnection(tcpConn), nil
}

// tcp 连接
type TcpConnection struct {
    *net.TCPConn
    protoBuffer []byte
}

// 创建 tcp 连接对象
func newTcpConnection(tcpConn *net.TCPConn) *TcpConnection {
    return &TcpConnection{TCPConn: tcpConn}
}

func (this *TcpConnection) ReadStream(stream []byte, count int) error {
    if len(stream) < count {
        return errors.New("bad stream")
    }
    stream = stream[:count]
    left := count
    for left > 0 {
        n, err := this.Read(stream)
        if n > 0 {
            left -= n
            if left > 0 {
                stream = stream[n:]
            }
        }
        if err != nil {
            return err
        }
    }
    return nil
}

const protoBufferLen = 8

func (this *TcpConnection) ReadProtoBuffer() error {
    this.protoBuffer = make([]byte, protoBufferLen)
    left := protoBufferLen
    for left > 0 {
        n, err := this.TCPConn.Read(this.protoBuffer)
        if n > 0 {
            left -= n
        }
        if err != nil {
            return err
        }
    }
    return nil
}

func (this *TcpConnection) IsHttpProto() bool {
    if this.protoBuffer == nil {
        return false
    }
    if this.protoBuffer[0] == 'G' && this.protoBuffer[1] == 'E' && this.protoBuffer[2] == 'T' && this.protoBuffer[3] == ' ' && this.protoBuffer[4] == '/' {
        return true
    }
    if this.protoBuffer[0] == 'P' && this.protoBuffer[1] == 'O' && this.protoBuffer[2] == 'S' && this.protoBuffer[3] == 'T' && this.protoBuffer[4] == ' ' && this.protoBuffer[5] == '/' {
        return true
    }
    return false
}

func (this *TcpConnection) Read(p []byte) (n int, err error) {
    if this.protoBuffer == nil {
        n, err = this.TCPConn.Read(p)
    } else {
        pLen := len(p)
        if pLen < protoBufferLen {
            copy(p, this.protoBuffer[0:pLen])
            this.protoBuffer = this.protoBuffer[pLen:]
            n = pLen
        } else {
            copy(p, this.protoBuffer)
            this.protoBuffer = nil
            n = protoBufferLen
        }
    }
    return
}

func (this *TcpConnection) ReadRpcRequest(timeout time.Duration) (*RpcRequest, error) {
    if err := this.SetReadDeadline(time.Now().Add(timeout)); err != nil {
        return nil, err
    }
    decoder := gob.NewDecoder(this)
    request := NewRpcRequest()
    if err := decoder.Decode(request); err != nil {
        return nil, err
    }
    return request, nil
}

func (this *TcpConnection) WriteRpcResponse(response *RpcResponse, timeout time.Duration) error {
    if err := this.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
        return err
    }
    encoder := gob.NewEncoder(this)
    if err := encoder.Encode(response); err != nil {
        return err
    }
    return nil
}

func (this *TcpConnection) Rpc(request *RpcRequest, readTimeout time.Duration, writeTimeout time.Duration) (*RpcResponse, error) {
    if err := this.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
        return nil, err
    }
    encoder := gob.NewEncoder(this)
    if err := encoder.Encode(request); err != nil {
        return nil, err
    }
    if err := this.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
        return nil, err
    }
    decoder := gob.NewDecoder(this)
    response := NewRpcResponse()
    if err := decoder.Decode(response); err != nil {
        return nil, err
    }
    return response, nil
}

// 注册 rpc 类型
func RegisterRpcTypeForValue(value interface{}) {
    gob.Register(value)
}

type RpcRequest struct {
    Func string
    Args map[string]interface{}
}

func NewRpcRequest() *RpcRequest {
    return &RpcRequest{Args: make(map[string]interface{})}
}

type RpcResponse struct {
    Result bool
    Reason string
    Code   int
    Args   map[string]interface{}
    Data   interface{}
}

func NewRpcResponse() *RpcResponse {
    return &RpcResponse{Args: make(map[string]interface{})}
}
