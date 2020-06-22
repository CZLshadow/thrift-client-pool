namespace java com.czl.api.thrift.model
namespace cpp com.czl.api
namespace php com.czl.api
namespace py com.czl.api
namespace js com.czl.apixianz
namespace go com.czl.api


struct ApiRequest {
    1: required i16 id;
}

struct  ApiResponse{
    1:required string name;
}

// service1
service ApiService1{
    ApiResponse query(1:ApiRequest request)
}

// service2
service ApiService2{
    ApiResponse query(1:ApiRequest request)
}
