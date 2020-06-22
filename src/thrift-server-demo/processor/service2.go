package processor

import (
	"context"
	"thrift-server-demo/com/czl/api"
)

type SimpleService2 struct {

}

func NewSimpleService2() *SimpleService2 {
	return new(SimpleService2)
}

func (s *SimpleService2) Query(ctx context.Context, request *api.ApiRequest) (r *api.ApiResponse, err error) {
	return nil, nil
}