package middleware

import (
	"context"
	"strings"

	"github.com/fekuna/omnipos-order-service/internal/auth"
	"github.com/fekuna/omnipos-pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthContextInterceptor extracts auth metadata and puts it in context
type AuthContextInterceptor struct {
	logger logger.ZapLogger
}

// NewAuthContextInterceptor creates a new auth context interceptor
func NewAuthContextInterceptor(log logger.ZapLogger) *AuthContextInterceptor {
	return &AuthContextInterceptor{
		logger: log,
	}
}

// isPublicEndpoint checks if the endpoint requires authentication
func (i *AuthContextInterceptor) isPublicEndpoint(method string) bool {
	// Add public endpoints here if any
	return false
}

// Unary returns a server interceptor that enriches context with user data
func (i *AuthContextInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if i.isPublicEndpoint(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			i.logger.Error("no metadata in request")
			return nil, status.Error(codes.Unauthenticated, "missing authentication context")
		}

		// Extract merchant ID (required)
		merchantIDs := md.Get("x-merchant-id")
		if len(merchantIDs) == 0 {
			i.logger.Error("no merchant ID in metadata")
			return nil, status.Error(codes.Unauthenticated, "missing merchant context")
		}

		// Build user context
		userCtx := &auth.UserContext{
			MerchantID: merchantIDs[0],
		}

		userIDs := md.Get("x-user-id")
		if len(userIDs) > 0 {
			userCtx.UserID = userIDs[0]
		}

		// Extract permissions
		perms := md.Get("x-permissions")
		if len(perms) > 0 {
			// Handle comma-separated list like "perm1,perm2"
			for _, p := range perms {
				userCtx.Permissions = append(userCtx.Permissions, splitPermissions(p)...)
			}
		}

		// Add to context
		ctx = auth.WithUserContext(ctx, userCtx)

		return handler(ctx, req)
	}
}

func splitPermissions(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
