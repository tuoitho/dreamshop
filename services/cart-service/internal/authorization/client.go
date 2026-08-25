// Package authorization provides the Cart adapter for Identity's private gRPC API.
package authorization

import (
	"context"
	"fmt"
	"time"

	identityv1 "ecommerce.local/contracts/go/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client interface {
	CheckPermission(context.Context, string, string) (bool, error)
}

type client struct {
	conn    *grpc.ClientConn
	service identityv1.AuthorizationServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*client, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("authorization timeout must be positive: %s", timeout)
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create authorization connection: %w", err)
	}
	return &client{conn: conn, service: identityv1.NewAuthorizationServiceClient(conn), timeout: timeout}, nil
}

func (c *client) Close() error { return c.conn.Close() }

func (c *client) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	response, err := c.service.CheckPermission(callCtx, &identityv1.CheckPermissionRequest{UserId: userID, Permission: permission})
	if err != nil {
		return false, err
	}
	return response.GetAllowed(), nil
}

func (c *client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}
