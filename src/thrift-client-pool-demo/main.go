package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/apache/thrift/lib/go/thrift"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"thrift-client-pool-demo/com/czl/api"
	"thrift-client-pool-demo/rpc/thrift/client/pool"
	"time"
)

var (
	addr            = "127.0.0.1:9444"
	connTimeout     = time.Second * 2
	idleTimeout     = time.Second * 120
	timeout         = time.Second * 10
	maxConn         = int32(100)
	service         = "com.czl.api.ApiService1$Client"
	bct             = context.Background()
	delay           int64 // 单位微妙
	successCount    = int64(0)
	failCount       = int64(0)
	thriftPoolAgent *pool.ThriftPoolAgent
)

// 初始化Thrift连接池代理
func init() {
	config := &pool.ThriftPoolConfig{
		Addr:        addr,
		MaxConn:     maxConn,
		ConnTimeout: connTimeout,
		IdleTimeout: idleTimeout,
		Timeout:     timeout,
	}
	thriftPool := pool.NewThriftPool(config, thriftDial, closeThriftClient)
	thriftPoolAgent = new(pool.ThriftPoolAgent)
	thriftPoolAgent.Init(thriftPool)
}

func thriftDial(addr string, connTimeout time.Duration) (*pool.IdleClient, error) {
	socket, err := thrift.NewTSocketTimeout(addr, connTimeout)
	if err != nil {
		return nil, err
	}
	transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	transport, err := transportFactory.GetTransport(socket)
	if err != nil {
		return nil, err
	}
	if err := transport.Open(); err != nil {
		return nil, err
	}

	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	iprot := thrift.NewTMultiplexedProtocol(protocolFactory.GetProtocol(transport), service)
	oprot := thrift.NewTMultiplexedProtocol(protocolFactory.GetProtocol(transport), service)

	client := api.NewApiService1ClientProtocol(transport, iprot, oprot)
	return &pool.IdleClient{
		Transport: transport,
		RawClient: client,
	}, nil
}

func closeThriftClient(c *pool.IdleClient) error {
	if c == nil {
		return nil
	}
	return c.Transport.Close()
}

func main() {
	go startPprof()
	for i := 0; i < 100; i++ {
		go start()
	}
	time.Sleep(time.Second * 600)
	avgQps := float64(successCount) / float64(600)
	avgDelay := float64(delay) / float64(successCount) / 1000
	log.Println(fmt.Sprintf("总运行时间：600s, 并发协程数：100，平均吞吐量：%v，平均延迟（ms）：%v，总成功数：%d，总失败数：%d",
		avgQps, avgDelay, successCount, failCount))
}

func startPprof() {
	http.ListenAndServe("0.0.0.0:9999", nil)
}

func start() {
	for {
		if resp, err := dialApi(); err != nil {
			log.Println(err)
		} else {
			log.Println(resp.Name)
		}
	}
}

func dialApi() (resp *api.ApiResponse, err error) {
	st := time.Now()
	err = thriftPoolAgent.Do(func(rawClient interface{}) error {
		client, ok := rawClient.(*api.ApiService1Client)
		if !ok {
			return errors.New("unknown client type")
		}
		var err2 error
		resp, err2 = client.Query(bct, &api.ApiRequest{
			ID: int16(rand.Intn(100)),
		})
		return err2
	})
	if err != nil {
		fail()
	} else {
		success(st)
	}
	return
}

func success(st time.Time) {
	atomic.AddInt64(&delay, time.Now().Sub(st).Microseconds())
	atomic.AddInt64(&successCount, 1)
}

func fail() {
	atomic.AddInt64(&failCount, 1)
}
