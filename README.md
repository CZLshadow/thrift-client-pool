
```cassandraql
.
├── README.md
├── api.thrift // Thrift IDL文件
└── src
    ├── thrift-client-nopool-demo // Thrift Client Demo，不使用连接池
    │   ├── com
    │   │   └── czl
    │   │       └── api
    │   │           ├── GoUnusedProtection__.go
    │   │           ├── api-consts.go
    │   │           ├── api.go
    │   │           ├── api_service1-remote
    │   │           │   └── api_service1-remote.go
    │   │           └── api_service2-remote
    │   │               └── api_service2-remote.go
    │   └── main.go
    ├── thrift-client-pool-demo // Thrift Client Demo，使用连接池
    │   ├── com
    │   │   └── czl
    │   │       └── api
    │   │           ├── GoUnusedProtection__.go
    │   │           ├── api-consts.go
    │   │           ├── api.go
    │   │           ├── api_service1-remote
    │   │           │   └── api_service1-remote.go
    │   │           └── api_service2-remote
    │   │               └── api_service2-remote.go
    │   ├── main.go
    │   └── rpc
    │       └── thrift
    │           └── client
    │               └── pool // Thrift客户端连接池的实现代码
    │                   ├── agent.go
    │                   └── pool.go
    └── thrift-server-demo // Thrift Server Demo
        ├── com
        │   └── czl
        │       └── api
        │           ├── GoUnusedProtection__.go
        │           ├── api-consts.go
        │           ├── api.go
        │           ├── api_service1-remote
        │           │   └── api_service1-remote.go
        │           └── api_service2-remote
        │               └── api_service2-remote.go
        ├── main.go
        └── processor
            ├── service1.go
            └── service2.go

```

thrift客户端连接池实现与测试demo
