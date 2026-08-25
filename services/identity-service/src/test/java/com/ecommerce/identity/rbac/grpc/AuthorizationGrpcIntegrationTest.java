package com.ecommerce.identity.rbac.grpc;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.contracts.identity.v1.AuthorizationServiceGrpc;
import com.ecommerce.contracts.identity.v1.CheckPermissionRequest;
import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class AuthorizationGrpcIntegrationTest extends PostgresIntegrationTestSupport {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000042");

    @Autowired
    GrpcServerLifecycle lifecycle;

    @Autowired
    JdbcTemplate jdbcTemplate;

    private ManagedChannel channel;

    @BeforeEach
    void setUp() {
        RbacTestData.resetDatabase(jdbcTemplate);
        jdbcTemplate.update("insert into identity_users "
                + "(id, email, normalized_email, password_hash, status, created_at, updated_at) "
                + "values (?, ?, ?, ?, 'ACTIVE', now(), now())", USER_ID,
                "grpc-customer@example.com", "grpc-customer@example.com", "hash");
        jdbcTemplate.update("insert into auth_assignments (user_id, item_name, created_at) values (?, 'CUSTOMER', now())",
                USER_ID);
        channel = ManagedChannelBuilder.forAddress("127.0.0.1", lifecycle.localPort())
                .usePlaintext()
                .build();
    }

    @AfterEach
    void tearDown() throws InterruptedException {
        if (channel != null) {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
        }
    }

    @Test
    void reflectsHierarchyRevocationOnTheSameChannel() {
        AuthorizationServiceGrpc.AuthorizationServiceBlockingStub stub =
                AuthorizationServiceGrpc.newBlockingStub(channel);
        CheckPermissionRequest request = CheckPermissionRequest.newBuilder()
                .setUserId(USER_ID.toString())
                .setPermission("cart.item.add")
                .build();

        assertThat(stub.checkPermission(request).getAllowed()).isTrue();

        jdbcTemplate.update("delete from auth_item_children "
                + "where parent_name = 'CUSTOMER' and child_name = 'cart.item.add'");

        assertThat(stub.checkPermission(request).getAllowed()).isFalse();
    }
}
