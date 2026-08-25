package com.ecommerce.identity.rbac.grpc;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.tuple;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

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
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

class AuthorizationGrpcServiceTest {
    private final RbacAuthorizationService rbac = Mockito.mock(RbacAuthorizationService.class);
    private final AuthorizationGrpcService service = new AuthorizationGrpcService(rbac);
    private final UUID userId = UUID.fromString("00000000-0000-0000-0000-000000000041");

    @Test
    void delegatesValidSingleRequest() {
        when(rbac.can(userId, "cart.item.add")).thenReturn(true);
        RecordingObserver<CheckPermissionResponse> observer = new RecordingObserver<>();

        service.checkPermission(CheckPermissionRequest.newBuilder()
                .setUserId(userId.toString())
                .setPermission("cart.item.add")
                .build(), observer);

        assertThat(observer.value().getAllowed()).isTrue();
        assertThat(observer.error()).isNull();
        assertThat(observer.completed()).isTrue();
        verify(rbac).can(userId, "cart.item.add");
    }

    @Test
    void returnsBatchDecisionsInOriginalOrderIncludingDuplicates() {
        List<String> permissions = List.of("cart.item.add", "cart.item.remove", "cart.item.add");
        when(rbac.allowedPermissions(userId, permissions)).thenReturn(Set.of("cart.item.add"));
        RecordingObserver<CheckPermissionsResponse> observer = new RecordingObserver<>();

        service.checkPermissions(CheckPermissionsRequest.newBuilder()
                .setUserId(userId.toString())
                .addAllPermissions(permissions)
                .build(), observer);

        assertThat(observer.value().getDecisionsList())
                .extracting(PermissionDecision::getPermission, PermissionDecision::getAllowed)
                .containsExactly(
                        tuple("cart.item.add", true),
                        tuple("cart.item.remove", false),
                        tuple("cart.item.add", true));
        assertThat(observer.error()).isNull();
        assertThat(observer.completed()).isTrue();
        verify(rbac).allowedPermissions(userId, permissions);
    }

    @Test
    void rejectsMalformedUuidWithoutCallingRbac() {
        assertInvalid(CheckPermissionRequest.newBuilder()
                .setUserId("not-a-uuid")
                .setPermission("cart.item.add")
                .build());
    }

    @Test
    void rejectsBlankPermissionWithoutCallingRbac() {
        assertInvalid(CheckPermissionRequest.newBuilder()
                .setUserId(userId.toString())
                .setPermission(" ")
                .build());
    }

    @Test
    void rejectsUntrimmedPermissionWithoutCallingRbac() {
        assertInvalid(CheckPermissionRequest.newBuilder()
                .setUserId(userId.toString())
                .setPermission(" cart.item.add")
                .build());
    }

    @Test
    void rejectsEmptyBatchWithoutCallingRbac() {
        assertInvalid(CheckPermissionsRequest.newBuilder().setUserId(userId.toString()).build());
    }

    @Test
    void rejectsBatchLargerThanOneHundredWithoutCallingRbac() {
        assertInvalid(CheckPermissionsRequest.newBuilder()
                .setUserId(userId.toString())
                .addAllPermissions(java.util.Collections.nCopies(101, "cart.item.add"))
                .build());
    }

    @Test
    void returnsSafeInternalErrorForUnexpectedApplicationFailure() {
        when(rbac.can(userId, "cart.item.add")).thenThrow(new IllegalStateException("database password"));
        RecordingObserver<CheckPermissionResponse> observer = new RecordingObserver<>();

        service.checkPermission(CheckPermissionRequest.newBuilder()
                .setUserId(userId.toString())
                .setPermission("cart.item.add")
                .build(), observer);

        Status status = Status.fromThrowable(observer.error());
        assertThat(status.getCode()).isEqualTo(Status.Code.INTERNAL);
        assertThat(status.getDescription()).isEqualTo("authorization check failed");
    }

    private void assertInvalid(CheckPermissionRequest request) {
        RecordingObserver<CheckPermissionResponse> observer = new RecordingObserver<>();

        service.checkPermission(request, observer);

        assertThat(Status.fromThrowable(observer.error()).getCode()).isEqualTo(Status.Code.INVALID_ARGUMENT);
        verify(rbac, never()).can(any(UUID.class), any());
    }

    private void assertInvalid(CheckPermissionsRequest request) {
        RecordingObserver<CheckPermissionsResponse> observer = new RecordingObserver<>();

        service.checkPermissions(request, observer);

        assertThat(Status.fromThrowable(observer.error()).getCode()).isEqualTo(Status.Code.INVALID_ARGUMENT);
        verify(rbac, never()).allowedPermissions(any(UUID.class), any());
    }

    private static final class RecordingObserver<T> implements StreamObserver<T> {
        private T value;
        private Throwable error;
        private boolean completed;

        @Override
        public void onNext(T value) {
            this.value = value;
        }

        @Override
        public void onError(Throwable throwable) {
            this.error = throwable;
        }

        @Override
        public void onCompleted() {
            this.completed = true;
        }

        T value() {
            return value;
        }

        Throwable error() {
            return error;
        }

        boolean completed() {
            return completed;
        }
    }
}
