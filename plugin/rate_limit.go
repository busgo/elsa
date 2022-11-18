package plugin

import (
	"context"
	"strings"

	"github.com/busgo/elsa/limit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RateLimitUnaryServerInterceptor() grpc.UnaryServerInterceptor {

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {

		serviceName := parseTargetServiceName(info.FullMethod)
		if serviceName == "" {
			return handler(ctx, req)
		}

		limiter := limit.Load(serviceName)
		if limiter == nil {
			return handler(ctx, req)
		}

		if !limiter.Take() {
			return nil, status.Errorf(codes.ResourceExhausted, "%s is rejected by rate limit, please retry again later", info.FullMethod)
		}
		defer limiter.Return()

		return handler(ctx, req)
	}
}

func RateLimitStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		serviceName := parseTargetServiceName(info.FullMethod)
		if serviceName == "" {
			return handler(srv, ss)
		}

		limiter := limit.Load(serviceName)
		if limiter == nil {
			return handler(srv, ss)
		}
		if !limiter.Take() {
			return status.Errorf(codes.ResourceExhausted, "%s is rejected by rate limit, please retry again later", info.FullMethod)
		}

		defer limiter.Return()
		return handler(srv, ss)
	}
}

func parseTargetServiceName(fullMethod string) string {
	target := strings.TrimPrefix(fullMethod, "/")
	return target[:strings.LastIndex(target, "/")]
}
