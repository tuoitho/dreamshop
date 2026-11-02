// Package authorization provides the Inventory adapter for Identity's private gRPC API.
package authorization

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	identityv1 "ecommerce.local/contracts/go/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service identityv1.AuthorizationServiceClient
	timeout time.Duration
}

func NewClient(target string, timeout time.Duration) (*Client, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("authorization target address is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("authorization timeout must be positive: %s", timeout)
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create authorization connection: %w", err)
	}
	return &Client{conn: conn, service: identityv1.NewAuthorizationServiceClient(conn), timeout: timeout}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.service.CheckPermission(callCtx, &identityv1.CheckPermissionRequest{
		UserId:     userID,
		Permission: permission,
	})
	if err != nil {
		return false, err
	}
	return response.GetAllowed(), nil
}

func (c *Client) CheckAnyPermission(ctx context.Context, userID string, permissions ...string) (bool, error) {
	if len(permissions) == 0 {
		return false, nil
	}
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.service.CheckPermissions(callCtx, &identityv1.CheckPermissionsRequest{
		UserId:      userID,
		Permissions: permissions,
	})
	if err != nil {
		return false, err
	}
	for _, decision := range response.GetDecisions() {
		if decision.GetAllowed() {
			return true, nil
		}
	}
	return false, nil
}
