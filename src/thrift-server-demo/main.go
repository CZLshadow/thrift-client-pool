package main

import (
	"github.com/apache/thrift/lib/go/thrift"
	"log"
	"thrift-server-demo/com/czl/api"
	"thrift-server-demo/processor"
)

func main() {
	startRPCServer()
}

func startRPCServer() {
	service1Proc := api.NewApiService1Processor(processor.NewSimpleService1())
	service2Proc := api.NewApiService2Processor(processor.NewSimpleService2())
	multiProc := thrift.NewTMultiplexedProcessor()
	multiProc.RegisterProcessor("com.czl.api.ApiService1$Client",
		service1Proc)
	multiProc.RegisterProcessor("com.czl.api.ApiService2$Client",
		service2Proc)
	serverSocket, err := thrift.NewTServerSocket("127.0.0.1:9444")
	if err != nil {
		log.Fatalln("err:", err)
	}
	transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	server := thrift.NewTSimpleServer4(multiProc, serverSocket, transportFactory, protocolFactory)
	server.Serve()
}