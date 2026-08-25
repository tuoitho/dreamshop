package com.ecommerce.identity.address.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.identity.address.domain.ShippingAddressNotFoundException;
import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.user.domain.InvalidProfileLocationException;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class ShippingAddressServiceTest extends PostgresIntegrationTestSupport {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000041");
    private static final UUID OTHER_USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000042");

    @Autowired
    ShippingAddressService service;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        RbacTestData.resetDatabase(jdbcTemplate);
        insertUser(USER_ID, "address-owner@example.com");
        insertUser(OTHER_USER_ID, "other-address-owner@example.com");
    }

    @Test
    void firstAddressIsDefaultAndSecondAddressIsNot() {
        ShippingAddressView first = service.create(USER_ID, command("First"));
        ShippingAddressView second = service.create(USER_ID, command("Second"));

        assertThat(first.isDefault()).isTrue();
        assertThat(second.isDefault()).isFalse();
        assertThat(service.list(USER_ID)).extracting(ShippingAddressView::isDefault)
                .containsExactly(true, false);
    }

    @Test
    void changingDefaultAndDeletingItPromotesTheNewestRemainingAddress() {
        ShippingAddressView first = service.create(USER_ID, command("First"));
        ShippingAddressView second = service.create(USER_ID, command("Second"));

        assertThat(service.setDefault(USER_ID, second.id()).isDefault()).isTrue();
        service.delete(USER_ID, second.id());

        assertThat(service.list(USER_ID)).hasSize(1);
        assertThat(service.list(USER_ID).getFirst().id()).isEqualTo(first.id());
        assertThat(service.list(USER_ID).getFirst().isDefault()).isTrue();
    }

    @Test
    void addressCannotBeReadOrChangedThroughAnotherUser() {
        ShippingAddressView address = service.create(USER_ID, command("Private"));

        assertThatThrownBy(() -> service.update(OTHER_USER_ID, address.id(),
                UpdateShippingAddressCommand.keepAll()))
                .isInstanceOf(ShippingAddressNotFoundException.class);
        assertThatThrownBy(() -> service.delete(OTHER_USER_ID, address.id()))
                .isInstanceOf(ShippingAddressNotFoundException.class);
    }

    @Test
    void rejectsWardThatDoesNotBelongToProvince() {
        assertThatThrownBy(() -> service.create(USER_ID, new CreateShippingAddressCommand(
                "Recipient", "0901234567", "79", "00043", "12 Le Loi", null)))
                .isInstanceOf(InvalidProfileLocationException.class)
                .satisfies(error -> assertThat(((InvalidProfileLocationException) error).errors())
                        .containsEntry("wardCode", "ward code does not belong to province code"));
    }

    @Test
    void concurrentFirstAddressCreationLeavesExactlyOneDefault() throws Exception {
        CountDownLatch ready = new CountDownLatch(2);
        ExecutorService executor = Executors.newFixedThreadPool(2);
        try {
            Future<?> first = executor.submit(() -> createAfterReady(ready, "Concurrent A"));
            Future<?> second = executor.submit(() -> createAfterReady(ready, "Concurrent B"));
            first.get();
            second.get();
        } finally {
            executor.shutdownNow();
        }

        assertThat(jdbcTemplate.queryForObject(
                "select count(*) from shipping_addresses where user_id = ? and is_default", Long.class, USER_ID))
                .isEqualTo(1L);
    }

    private void createAfterReady(CountDownLatch ready, String label) {
        ready.countDown();
        try {
            ready.await();
            service.create(USER_ID, command(label));
        } catch (InterruptedException exception) {
            Thread.currentThread().interrupt();
            throw new AssertionError(exception);
        }
    }

    private CreateShippingAddressCommand command(String label) {
        return new CreateShippingAddressCommand(label, "0901234567", "79", "26740",
                "12 Le Loi", label);
    }

    private void insertUser(UUID id, String email) {
        jdbcTemplate.update("""
                insert into identity_users
                    (id, email, normalized_email, password_hash, status, created_at, updated_at)
                values (?, ?, ?, 'hash', 'ACTIVE', now(), now())
                """, id, email, email);
        jdbcTemplate.update("insert into identity_profiles (user_id, full_name) values (?, ?)", id, email);
    }
}
