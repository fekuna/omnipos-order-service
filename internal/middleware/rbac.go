package middleware

import (
	"context"

	"github.com/fekuna/omnipos-order-service/internal/auth"
	"github.com/fekuna/omnipos-pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RBACInterceptor struct {
	logger        logger.ZapLogger
	permissionMap map[string]string // Method -> Required Permission
}

func NewRBACInterceptor(log logger.ZapLogger) *RBACInterceptor {
	// Define required permissions for each method
	permMap := map[string]string{
		"/order.v1.OrderService/CreateOrder": "order:write",
		"/order.v1.OrderService/GetOrder":    "order:read",
		"/order.v1.OrderService/ListOrders":  "order:read",
		"/order.v1.OrderService/UpdateOrder": "order:write",
		// Add other methods as needed
	}

	return &RBACInterceptor{
		logger:        log,
		permissionMap: permMap,
	}
}

func (i *RBACInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		requiredPerm, ok := i.permissionMap[info.FullMethod]
		if !ok {
			// If not in map, default to allow OR deny?
			// For safety, maybe allow if it's public, but deny if unknown?
			// Usually "Allow by default" for internal RPCs, but "Deny" for sensitive.
			// Given this is a specific RBAC interceptor, if we don't list it, we assume no specific permission needed (maybe just Authenticated).
			return handler(ctx, req)
		}

		userCtx := auth.GetUserContext(ctx)
		if userCtx == nil {
			return nil, status.Error(codes.Unauthenticated, "missing user context")
		}

		// Merchant Owner (no UserID) usually has full access?
		// Or we check permissions strictly.
		// If UserID is empty, it means it's a Merchant Token (Super Admin).
		// We should allow Merchant Owners to do everything.
		if userCtx.UserID == "" && userCtx.MerchantID != "" {
			// It's a merchant login, allow all
			return handler(ctx, req)
		}

		// Check permissions
		hasPerm := false
		for _, p := range userCtx.Permissions {
			if p == requiredPerm {
				hasPerm = true
				break
			}
		}

		if !hasPerm {
			return nil, status.Error(codes.PermissionDenied, "missing required permission: "+requiredPerm)
		}

		return handler(ctx, req)
	}
}
