package com.ecommerce.identity.address.persistence;

import com.ecommerce.identity.address.application.CreateShippingAddressCommand;
import com.ecommerce.identity.address.application.UpdateShippingAddressCommand;
import com.ecommerce.identity.address.domain.ShippingAddressPatch;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;
import java.util.Objects;
import java.util.UUID;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;

@Entity
@Table(name = "shipping_addresses")
@Getter
@Accessors(fluent = true)
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class ShippingAddressEntity {
    @Id
    @Column(nullable = false, updatable = false)
    private UUID id;

    @Column(name = "user_id", nullable = false, updatable = false)
    private UUID userId;

    @Column(name = "recipient_name", nullable = false, length = 100)
    private String recipientName;

    @Column(nullable = false, length = 20)
    private String phone;

    @Column(name = "province_code", nullable = false, length = 20)
    private String provinceCode;

    @Column(name = "ward_code", nullable = false, length = 20)
    private String wardCode;

    @Column(name = "address_line", nullable = false, length = 255)
    private String addressLine;

    @Column(length = 50)
    private String label;

    @Column(name = "is_default", nullable = false)
    private boolean isDefault;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt;

    public static ShippingAddressEntity create(UUID id, UUID userId, CreateShippingAddressCommand command,
            boolean defaultAddress, Instant now) {
        ShippingAddressEntity entity = new ShippingAddressEntity();
        entity.id = Objects.requireNonNull(id, "id");
        entity.userId = Objects.requireNonNull(userId, "userId");
        entity.recipientName = Objects.requireNonNull(command.recipientName(), "recipientName");
        entity.phone = Objects.requireNonNull(command.phone(), "phone");
        entity.provinceCode = Objects.requireNonNull(command.provinceCode(), "provinceCode");
        entity.wardCode = Objects.requireNonNull(command.wardCode(), "wardCode");
        entity.addressLine = Objects.requireNonNull(command.addressLine(), "addressLine");
        entity.label = command.label();
        entity.isDefault = defaultAddress;
        entity.createdAt = Objects.requireNonNull(now, "now");
        entity.updatedAt = now;
        return entity;
    }

    public void apply(UpdateShippingAddressCommand command, Instant now) {
        recipientName = applyRequired(command.recipientName(), recipientName);
        phone = applyRequired(command.phone(), phone);
        provinceCode = applyRequired(command.provinceCode(), provinceCode);
        wardCode = applyRequired(command.wardCode(), wardCode);
        addressLine = applyRequired(command.addressLine(), addressLine);
        label = applyOptional(command.label(), label);
        updatedAt = Objects.requireNonNull(now, "now");
    }

    public void markDefault(Instant now) {
        isDefault = true;
        updatedAt = Objects.requireNonNull(now, "now");
    }

    public void clearDefault(Instant now) {
        isDefault = false;
        updatedAt = Objects.requireNonNull(now, "now");
    }

    private static String applyRequired(ShippingAddressPatch<String> patch, String current) {
        return switch (patch.action()) {
            case KEEP -> current;
            case SET -> patch.value();
            case CLEAR -> throw new IllegalArgumentException("Required shipping address field cannot be cleared");
        };
    }

    private static String applyOptional(ShippingAddressPatch<String> patch, String current) {
        return switch (patch.action()) {
            case KEEP -> current;
            case SET -> patch.value();
            case CLEAR -> null;
        };
    }
}
