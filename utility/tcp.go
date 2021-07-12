/**
 * Koala Rule Engine Core
 *
 * @package: main
 * @desc: koala engine tcp server
 *
 * @author: heiyeluren
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */

package utility

import (
	"encoding/gob"
	"errors"
	"net"
	"time"
)

// TcpListen 监听一个 tcp 地址
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

// TcpListener tcp 监听器
type TcpListener struct {
	*net.TCPListener
}

func newTcpListener(tcpListener *net.TCPListener) *TcpListener {
	return &TcpListener{TCPListener: tcpListener}
}

// Accept .
func (l *TcpListener) Accept() (*TcpConnection, error) {
	tcpConn, err := l.TCPListener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	return newTcpConnection(tcpConn), nil
}

// TcpConnect 连接一个 tcp 地址
func TcpConnect(localAddress string, remoteAddress string, timeout time.Duration) (*TcpConnection, error) {
	// @todo: 本 api 调用不绑定 localAddress，可能会影响上层应用程序
	conn, err := net.DialTimeout("tcp", remoteAddress, timeout)
	if err != nil {
		return nil, err
	}
	tcpConn := conn.(*net.TCPConn)
	return newTcpConnection(tcpConn), nil
}

// TcpConnection tcp 连接
type TcpConnection struct {
	*net.TCPConn
	protoBuffer []byte
}

// 创建 tcp 连接对象
func newTcpConnection(tcpConn *net.TCPConn) *TcpConnection {
	return &TcpConnection{TCPConn: tcpConn}
}

// ReadStream .
func (c *TcpConnection) ReadStream(stream []byte, count int) error {
	if len(stream) < count {
		return errors.New("bad stream")
	}
	stream = stream[:count]
	left := count
	for left > 0 {
		n, err := c.Read(stream)
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

// ReadProtoBuffer .
func (c *TcpConnection) ReadProtoBuffer() error {
	c.protoBuffer = make([]byte, protoBufferLen)
	left := protoBufferLen
	for left > 0 {
		n, err := c.TCPConn.Read(c.protoBuffer)
		if n > 0 {
			left -= n
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// IsHttpProto .
func (c *TcpConnection) IsHttpProto() bool {
	if c.protoBuffer == nil {
		return false
	}
	if c.protoBuffer[0] == 'G' && c.protoBuffer[1] == 'E' && c.protoBuffer[2] == 'T' && c.protoBuffer[3] == ' ' && c.protoBuffer[4] == '/' {
		return true
	}
	if c.protoBuffer[0] == 'P' && c.protoBuffer[1] == 'O' && c.protoBuffer[2] == 'S' && c.protoBuffer[3] == 'T' && c.protoBuffer[4] == ' ' && c.protoBuffer[5] == '/' {
		return true
	}
	return false
}

// Read .
func (c *TcpConnection) Read(p []byte) (n int, err error) {
	if c.protoBuffer == nil {
		n, err = c.TCPConn.Read(p)
	} else {
		pLen := len(p)
		if pLen < protoBufferLen {
			copy(p, c.protoBuffer[0:pLen])
			c.protoBuffer = c.protoBuffer[pLen:]
			n = pLen
		} else {
			copy(p, c.protoBuffer)
			c.protoBuffer = nil
			n = protoBufferLen
		}
	}
	return
}

// ReadRpcRequest .
func (c *TcpConnection) ReadRpcRequest(timeout time.Duration) (*RpcRequest, error) {
	if err := c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	decoder := gob.NewDecoder(c)
	request := NewRpcRequest()
	if err := decoder.Decode(request); err != nil {
		return nil, err
	}
	return request, nil
}

// WriteRpcResponse .
func (c *TcpConnection) WriteRpcResponse(response *RpcResponse, timeout time.Duration) error {
	if err := c.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	encoder := gob.NewEncoder(c)
	if err := encoder.Encode(response); err != nil {
		return err
	}
	return nil
}

// Rpc .
func (c *TcpConnection) Rpc(request *RpcRequest, readTimeout time.Duration, writeTimeout time.Duration) (*RpcResponse, error) {
	if err := c.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return nil, err
	}
	encoder := gob.NewEncoder(c)
	if err := encoder.Encode(request); err != nil {
		return nil, err
	}
	if err := c.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, err
	}
	decoder := gob.NewDecoder(c)
	response := NewRpcResponse()
	if err := decoder.Decode(response); err != nil {
		return nil, err
	}
	return response, nil
}

// RegisterRpcTypeForValue 注册 rpc 类型
func RegisterRpcTypeForValue(value interface{}) {
	gob.Register(value)
}

// RpcRequest .
type RpcRequest struct {
	Func string
	Args map[string]interface{}
}

// NewRpcRequest .
func NewRpcRequest() *RpcRequest {
	return &RpcRequest{Args: make(map[string]interface{})}
}

// RpcResponse .
type RpcResponse struct {
	Result bool
	Reason string
	Code   int
	Args   map[string]interface{}
	Data   interface{}
}

// NewRpcResponse .
func NewRpcResponse() *RpcResponse {
	return &RpcResponse{Args: make(map[string]interface{})}
}
