# Local development

For local development, run infrastructure dependencies with Docker Compose and run application services directly on Windows from a terminal or IDE. Application services include BFF, Identity, Catalog, Inventory, Order, and similar business services.
When Kong runs in Docker and proxies to an application service running on Windows, configure the Kong upstream URL with `host.docker.internal` and the service's Windows port.