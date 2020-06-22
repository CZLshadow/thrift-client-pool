package processor

import (
	"context"
	"math/rand"
	"strconv"
	"thrift-server-demo/com/czl/api"
)

type SimpleService1 struct {

}

func NewSimpleService1() *SimpleService1 {
	return new(SimpleService1)
}

func (s *SimpleService1) Query(ctx context.Context, request *api.ApiRequest) (r *api.ApiResponse, err error) {
	resp := api.ApiResponse{
		Name: "czl" + strconv.Itoa(rand.Intn(10000)),
	}
	return &resp, nil
}
