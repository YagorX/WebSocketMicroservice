package auth

import (
	"context"
	"fmt"
	"time"

	ssov1 "github.com/YagorX/protos/gen/go/sso"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"golang.org/x/exp/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	grpcClient ssov1.AuthClient
	// log        *slog.Logger
}

func New(ctx context.Context,
	log *slog.Logger,
	addr string,
	timeout time.Duration,
	retriesCount int,
) (*Client, error) {
	const op = "grpc.New"

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadReceived, grpclog.PayloadSent),
	}

	cc, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		grpcClient: ssov1.NewAuthClient(cc),
	}, nil
}

func (c *Client) ValidateToken(ctx context.Context, token string) (int64, error) {
	const op = "grpc.ValidateToken"
	resp, err := c.grpcClient.ValidateToken(context.Background(), &ssov1.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return resp.GetUserId(), nil
}

func InterceptorLogger(log *slog.Logger) grpclog.Logger {
	return grpclog.LoggerFunc(func(ctx context.Context, lvl grpclog.Level, msg string, fields ...any) {
		log.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}
