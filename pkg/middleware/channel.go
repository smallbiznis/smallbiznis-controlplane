package middleware

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Key type biar aman di context (tidak bentrok)
type channelKey struct{}

var ChannelContextKey = channelKey{}

// deriveChannelFromAPIKey menebak channel dari pola API key
func deriveChannelFromAPIKey(key string) string {
	switch {
	case strings.HasPrefix(key, "pos_"):
		return "pos"
	case strings.HasPrefix(key, "web_"):
		return "online"
	case strings.HasPrefix(key, "partner_"):
		return "partner"
	default:
		return "api"
	}
}

// ChannelInterceptor adalah unary interceptor gRPC
// yang menambahkan channel ke context berdasarkan x-api-key
func ChannelInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return handler(ctx, req)
		}

		keys := md.Get("x-api-key")
		channel := "api"
		if len(keys) > 0 {
			channel = deriveChannelFromAPIKey(keys[0])
		}

		ctx = context.WithValue(ctx, ChannelContextKey, channel)
		return handler(ctx, req)
	}
}

// FromChannel memeriksa apakah context berasal dari channel tertentu
func FromChannel(ctx context.Context, want string) bool {
	ch, ok := ctx.Value(ChannelContextKey).(string)
	return ok && ch == want
}

// GetChannel mengembalikan string channel saat ini (default "api")
func GetChannel(ctx context.Context) string {
	ch, ok := ctx.Value(ChannelContextKey).(string)
	if !ok {
		return "api"
	}
	return ch
}
