package com.ecommerce.identity.rbac.grpc;

import com.ecommerce.contracts.identity.v1.AuthorizationServiceGrpc.AuthorizationServiceImplBase;
import com.ecommerce.contracts.identity.v1.CheckPermissionRequest;
import com.ecommerce.contracts.identity.v1.CheckPermissionResponse;
import com.ecommerce.contracts.identity.v1.CheckPermissionsRequest;
import com.ecommerce.contracts.identity.v1.CheckPermissionsResponse;
import com.ecommerce.contracts.identity.v1.PermissionDecision;
import com.ecommerce.identity.rbac.application.RbacAuthorizationService;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.stereotype.Service;

@Service
public class AuthorizationGrpcService extends AuthorizationServiceImplBase {
    private static final Logger log = LoggerFactory.getLogger(AuthorizationGrpcService.class);
    private static final int MAX_BATCH_SIZE = 100;

    private final RbacAuthorizationService rbac;

    public AuthorizationGrpcService(RbacAuthorizationService rbac) {
        this.rbac = rbac;
    }

    @Override
    public void checkPermission(CheckPermissionRequest request,
            StreamObserver<CheckPermissionResponse> responseObserver) {
        try {
            UUID userId = requireUserId(request.getUserId());
            String permission = requirePermission(request.getPermission());
            boolean allowed = rbac.can(userId, permission);
            log.debug("gRPC authorization decision={} userId={} permission={} traceId={}",
                    allowed ? "allow" : "deny", userId, permission, MDC.get("trace_id"));
            responseObserver.onNext(CheckPermissionResponse.newBuilder().setAllowed(allowed).build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException exception) {
            responseObserver.onError(exception);
        } catch (RuntimeException exception) {
            log.warn("gRPC authorization provider failure operation=single traceId={}",
                    MDC.get("trace_id"), exception);
            responseObserver.onError(Status.INTERNAL.withDescription("authorization check failed")
                    .asRuntimeException());
        }
    }

    @Override
    public void checkPermissions(CheckPermissionsRequest request,
            StreamObserver<CheckPermissionsResponse> responseObserver) {
        try {
            UUID userId = requireUserId(request.getUserId());
            List<String> permissions = request.getPermissionsList();
            requireBatchSize(permissions.size());
            permissions.forEach(this::requirePermission);

            Set<String> allowedPermissions = rbac.allowedPermissions(userId, permissions);
            CheckPermissionsResponse.Builder response = CheckPermissionsResponse.newBuilder();
            permissions.forEach(permission -> response.addDecisions(PermissionDecision.newBuilder()
                    .setPermission(permission)
                    .setAllowed(allowedPermissions.contains(permission))));
            log.debug("gRPC authorization batch decision userId={} batchSize={} traceId={}",
                    userId, permissions.size(), MDC.get("trace_id"));
            responseObserver.onNext(response.build());
            responseObserver.onCompleted();
        } catch (StatusRuntimeException exception) {
            responseObserver.onError(exception);
        } catch (RuntimeException exception) {
            log.warn("gRPC authorization provider failure operation=batch traceId={}",
                    MDC.get("trace_id"), exception);
            responseObserver.onError(Status.INTERNAL.withDescription("authorization check failed")
                    .asRuntimeException());
        }
    }

    private UUID requireUserId(String rawUserId) {
        try {
            return UUID.fromString(rawUserId);
        } catch (RuntimeException exception) {
            throw Status.INVALID_ARGUMENT.withDescription("user_id must be a UUID")
                    .asRuntimeException();
        }
    }

    private String requirePermission(String permission) {
        if (permission == null || permission.isBlank() || !permission.equals(permission.trim())) {
            throw Status.INVALID_ARGUMENT.withDescription("permission must be non-blank and trimmed")
                    .asRuntimeException();
        }
        return permission;
    }

    private void requireBatchSize(int size) {
        if (size < 1 || size > MAX_BATCH_SIZE) {
            throw Status.INVALID_ARGUMENT.withDescription("permissions must contain 1 through 100 items")
                    .asRuntimeException();
        }
    }
}
