module ecommerce.local/cart-service

go 1.24

require (
	ecommerce.local/contracts/go v0.0.0
	github.com/jackc/pgx/v5 v5.7.2
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157
	google.golang.org/grpc v1.65.0
)

replace ecommerce.local/contracts/go => ../../contracts/go
