package com.ecommerce.identity.address.application;

import com.ecommerce.identity.address.domain.ShippingAddressNotFoundException;
import com.ecommerce.identity.address.domain.ShippingAddressPatch;
import com.ecommerce.identity.address.persistence.ShippingAddressEntity;
import com.ecommerce.identity.address.persistence.ShippingAddressRepository;
import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.location.LocationCatalog;
import com.ecommerce.identity.user.domain.InvalidProfileLocationException;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class ShippingAddressService {
    private final ShippingAddressRepository addresses;
    private final IdentityUserRepository users;
    private final LocationCatalog locationCatalog;
    private final Clock clock;

    @Transactional(readOnly = true)
    public List<ShippingAddressView> list(UUID userId) {
        users.findWithProfileById(userId)
                .filter(user -> user.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        return addresses.findAllByUserIdOrderByIsDefaultDescCreatedAtDescIdAsc(userId).stream()
                .map(this::view)
                .toList();
    }

    @Transactional
    public ShippingAddressView create(UUID userId, CreateShippingAddressCommand command) {
        requireActiveOwnerForUpdate(userId);
        String provinceCode = required(command.provinceCode(), "provinceCode");
        String wardCode = required(command.wardCode(), "wardCode");
        validateLocation(provinceCode, wardCode);
        CreateShippingAddressCommand normalized = new CreateShippingAddressCommand(
                required(command.recipientName(), "recipientName"),
                required(command.phone(), "phone"),
                provinceCode,
                wardCode,
                required(command.addressLine(), "addressLine"),
                command.label());
        Instant now = clock.instant();
        ShippingAddressEntity entity = ShippingAddressEntity.create(UUID.randomUUID(), userId, normalized,
                !addresses.existsByUserId(userId), now);
        return view(addresses.saveAndFlush(entity));
    }

    @Transactional
    public ShippingAddressView update(UUID userId, UUID addressId, UpdateShippingAddressCommand command) {
        requireActiveOwnerForUpdate(userId);
        ShippingAddressEntity address = ownedAddress(userId, addressId);
        String provinceCode = resolveRequired(command.provinceCode(), address.provinceCode(), "provinceCode");
        String wardCode = resolveRequired(command.wardCode(), address.wardCode(), "wardCode");
        validateLocation(provinceCode, wardCode);
        address.apply(new UpdateShippingAddressCommand(
                command.recipientName(), command.phone(), ShippingAddressPatch.set(provinceCode),
                ShippingAddressPatch.set(wardCode), command.addressLine(), command.label()), clock.instant());
        return view(addresses.saveAndFlush(address));
    }

    @Transactional
    public void delete(UUID userId, UUID addressId) {
        requireActiveOwnerForUpdate(userId);
        ShippingAddressEntity address = ownedAddress(userId, addressId);
        boolean wasDefault = address.isDefault();
        addresses.delete(address);
        addresses.flush();
        if (wasDefault) {
            addresses.findFirstByUserIdOrderByCreatedAtDescIdAsc(userId).ifPresent(replacement -> {
                replacement.markDefault(clock.instant());
                addresses.saveAndFlush(replacement);
            });
        }
    }

    @Transactional
    public ShippingAddressView setDefault(UUID userId, UUID addressId) {
        requireActiveOwnerForUpdate(userId);
        ShippingAddressEntity target = ownedAddress(userId, addressId);
        if (target.isDefault()) {
            return view(target);
        }
        addresses.findFirstByUserIdAndIsDefaultTrue(userId).ifPresent(current -> {
            current.clearDefault(clock.instant());
            addresses.saveAndFlush(current);
        });
        target.markDefault(clock.instant());
        return view(addresses.saveAndFlush(target));
    }

    private IdentityUserEntity requireActiveOwnerForUpdate(UUID userId) {
        return users.findForUpdateById(userId)
                .filter(user -> user.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
    }

    private ShippingAddressEntity ownedAddress(UUID userId, UUID addressId) {
        return addresses.findByIdAndUserId(addressId, userId)
                .orElseThrow(ShippingAddressNotFoundException::new);
    }

    private void validateLocation(String provinceCode, String wardCode) {
        if (locationCatalog.province(provinceCode).isEmpty()) {
            throw new InvalidProfileLocationException(Map.of("provinceCode", "unknown province code"));
        }
        if (!locationCatalog.wardBelongsToProvince(provinceCode, wardCode)) {
            throw new InvalidProfileLocationException(Map.of(
                    "wardCode", "ward code does not belong to province code"));
        }
    }

    private static String resolveRequired(ShippingAddressPatch<String> patch, String current, String field) {
        return switch (patch.action()) {
            case KEEP -> current;
            case SET -> required(patch.value(), field);
            case CLEAR -> throw new InvalidProfileLocationException(Map.of(field, "must not be blank"));
        };
    }

    private static String required(String value, String field) {
        if (value == null || value.trim().isEmpty()) {
            throw new InvalidProfileLocationException(Map.of(field, "must not be blank"));
        }
        return value.trim();
    }

    private ShippingAddressView view(ShippingAddressEntity address) {
        return new ShippingAddressView(address.id(), address.recipientName(), address.phone(),
                address.provinceCode(), address.wardCode(), address.addressLine(), address.label(),
                address.isDefault(), address.createdAt(), address.updatedAt());
    }
}
