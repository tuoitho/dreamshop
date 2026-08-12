#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"

export IDENTITY_JWT_PRIVATE_KEY_PATH="$repo_root/deployments/secrets/identity/private.pem"
export IDENTITY_JWT_PUBLIC_KEY_PATH="$repo_root/deployments/secrets/identity/public.pem"
export SERVER_PORT="8080"
export IDENTITY_JWT_ISSUER="http://localhost:8080"
export IDENTITY_SESSION_COOKIE_SECURE="false"
# Docker Desktop's host.docker.internal gateway forwards to host IPv4 only;
# force Tomcat onto an IPv4 socket so Kong can reach the service.
export JDK_JAVA_OPTIONS="-Djava.net.preferIPv4Stack=true"

cd "$repo_root"
mvn -f services/identity-service/pom.xml spring-boot:run
