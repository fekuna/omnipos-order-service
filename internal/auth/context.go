package auth

import "context"

// UserContext represents authenticated user information extracted from request metadata
type UserContext struct {
	MerchantID  string
	UserID      string
	Email       string
	Role        string
	Permissions []string // RBAC Permissions
}

// Context key type for type safety
type contextKey string

const userContextKey contextKey = "user_context"

// WithUserContext adds user context to the context
func WithUserContext(ctx context.Context, userCtx *UserContext) context.Context {
	return context.WithValue(ctx, userContextKey, userCtx)
}

// GetUserContext extracts user context from context
func GetUserContext(ctx context.Context) *UserContext {
	userCtx, ok := ctx.Value(userContextKey).(*UserContext)
	if !ok {
		return nil
	}
	return userCtx
}

// GetMerchantID is a convenience method to get merchant ID from context
func GetMerchantID(ctx context.Context) string {
	userCtx := GetUserContext(ctx)
	if userCtx == nil {
		return ""
	}
	return userCtx.MerchantID
}
