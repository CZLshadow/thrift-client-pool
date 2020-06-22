package pool

import (
	"fmt"
	"github.com/apache/thrift/lib/go/thrift"
	"log"
	"net"
)

type ThriftPoolAgent struct {
	pool *ThriftPool
}

func NewThriftPoolAgent() *ThriftPoolAgent {
	return &ThriftPoolAgent{}
}

func (a *ThriftPoolAgent) Init(pool *ThriftPool) {
	a.pool = pool
}

// 真正的业务逻辑放到do方法做，ThriftPoolAgent只要保证获取到可用的Thrift客户端，然后传给do方法就行了
func (a *ThriftPoolAgent) Do(do func(rawClient interface{}) error) error {
	var (
		client *IdleClient
		err error
	)
	defer func() {
		if client != nil {
			if err == nil {
				if rErr := a.releaseClient(client); rErr != nil {
					log.Println(fmt.Sprintf("releaseClient error: %v", rErr))
				}
			} else if _, ok := err.(net.Error); ok {
				a.closeClient(client)
			} else if _, ok = err.(thrift.TTransportException); ok {
				a.closeClient(client)
			} else {
				if rErr := a.releaseClient(client); rErr != nil {
					log.Println(fmt.Sprintf("releaseClient error: %v", rErr))
				}
			}
		}
	}()
	// 从连接池里获取链接
	client, err = a.getClient()
	if err != nil {
		return err
	}
	if err = do(client.RawClient); err != nil {
		if _, ok := err.(net.Error); ok {
			log.Println(fmt.Sprintf("err: retry tcp, %T, %s", err, err.Error()))
			// 网络错误，重建连接
			client, err = a.reconnect(client)
			if err != nil {
				return err
			}
			return do(client.RawClient)
		}

		if _, ok := err.(thrift.TTransportException); ok {
			log.Println(fmt.Sprintf("err: retry tcp, %T, %s", err, err.Error()))
			// thrift传输层错误，也重建连接
			client, err = a.reconnect(client)
			if err != nil {
				return err
			}
			return do(client.RawClient)
		}
		return err
	}
	return nil
}

// 获取连接
func (a *ThriftPoolAgent) getClient() (*IdleClient, error) {
	return a.pool.Get()
}

// 释放连接
func (a *ThriftPoolAgent) releaseClient(client *IdleClient) error {
	return a.pool.Put(client)
}

// 关闭有问题的连接，并重新创建一个新的连接
func (a *ThriftPoolAgent) reconnect(client *IdleClient) (newClient *IdleClient, err error) {
	return a.pool.Reconnect(client)
}

// 关闭连接
func (a *ThriftPoolAgent) closeClient(client *IdleClient) {
	a.pool.CloseConn(client)
}

// 释放连接池
func (a *ThriftPoolAgent) Release() {
	a.pool.Release()
}

func (a *ThriftPoolAgent) GetIdleCount() uint32 {
	return a.pool.GetIdleCount()
}

func (a *ThriftPoolAgent) GetConnCount() int32 {
	return a.pool.GetConnCount()
}