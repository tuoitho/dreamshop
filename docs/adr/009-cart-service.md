# ADR-009: Cart service ownership

## Status

Accepted

## Decision

Add `cart-service` (Go) as the owner of authenticated user cart lines. It owns
`cart_db` and validates a cart view through Catalog and Inventory gRPC contracts.
Catalog remains the owner of product/price/deleted facts; Inventory remains the
owner of availability and reservations; Order remains the owner of order state;
payment remains outside Cart ownership. The BFF exposes authenticated REST
adaptation through Kong. Cart selection stays in the browser and is not persisted
by Cart.