package glink

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

func retryInterceptor(cm *ConnectionManager) grpc.UnaryClientInterceptor {

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var err error

		for attempt := 0; attempt < cm.maxRetries; attempt++ {
			select {
			case <-ctx.Done():
				return ctx.Err() // If the context is expired, return immediately

			default:
				var newConn *grpc.ClientConn = nil

				if newConn == nil {
					err = invoker(ctx, method, req, reply, cc, opts...)
				} else {
					err = invoker(ctx, method, req, reply, newConn, opts...)
				}

				// If successful, return without retrying
				if err == nil {
					return nil
				}

				// Check if the error is a timeout or transient failure
				if status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Unavailable {

					select {
					case <-time.After(cm.backoffDuration(attempt)): // Wait before retrying
					case <-ctx.Done(): // If context expires during wait, exit early
						return ctx.Err()
					}

					// Force reconnection if newtwork error
					GetLogger().Printf("Retrying request (attempt %d/%d) due to error: %v", attempt+1, cm.maxRetries, err)
					newConn, _ = cm.TryReconnect()

					continue
				}

				// If it's a non-retriable error, return immediately
				break
			}
		}

		return err
	}
}
