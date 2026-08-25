package com.ecommerce.identity.user.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.location.LocationCatalog;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.domain.Gender;
import com.ecommerce.identity.user.domain.InvalidProfileLocationException;
import com.ecommerce.identity.user.domain.ProfilePatch;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.LocalDate;
import java.util.Map;
import java.util.UUID;
import java.util.function.Function;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class MeService {
    private final IdentityUserRepository repository;
    private final Clock clock;
    private final LocationCatalog locationCatalog;
    private final RbacRepository rbac;

    public MeService(IdentityUserRepository repository, Clock clock, LocationCatalog locationCatalog,
            RbacRepository rbac) {
        this.repository = repository;
        this.clock = clock;
        this.locationCatalog = locationCatalog;
        this.rbac = rbac;
    }

    public CurrentUser current(UUID userId) {
        IdentityUserEntity user = repository.findWithProfileById(userId)
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        return toCurrentUser(user, rbac.assignedRoles(userId));
    }

    @Transactional
    public CurrentUser update(UUID userId, String fullName, String phone, String dateOfBirth,
            String gender, String provinceCode, String wardCode, String addressLine) {
        IdentityUserEntity user = repository.findWithProfileById(userId)
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        ProfilePatch<String> provincePatch = patch(provinceCode, Function.identity());
        ProfilePatch<String> wardPatch = patch(wardCode, Function.identity());
        ResolvedLocation location = resolveLocation(provincePatch, wardPatch);
        user.updateProfile(fullName == null ? ProfilePatch.keep() : ProfilePatch.set(fullName),
                patch(phone, Function.identity()),
                patch(dateOfBirth, LocalDate::parse),
                patch(gender, Gender::valueOf),
                location.provinceCode(),
                location.wardCode(),
                patch(addressLine, Function.identity()),
                clock.instant());
        return toCurrentUser(user, rbac.assignedRoles(userId));
    }

    private static <T> ProfilePatch<T> patch(String raw, Function<String, T> parse) {
        if (raw == null) {
            return ProfilePatch.keep();
        }
        String trimmed = raw.trim();
        if (trimmed.isEmpty()) {
            return ProfilePatch.clear();
        }
        return ProfilePatch.set(parse.apply(trimmed));
    }

    private ResolvedLocation resolveLocation(ProfilePatch<String> provincePatch, ProfilePatch<String> wardPatch) {
        if (provincePatch.action() == ProfilePatch.Action.CLEAR
                || wardPatch.action() == ProfilePatch.Action.CLEAR) {
            return new ResolvedLocation(ProfilePatch.clear(), ProfilePatch.clear());
        }
        if (provincePatch.action() == ProfilePatch.Action.KEEP
                && wardPatch.action() == ProfilePatch.Action.KEEP) {
            return new ResolvedLocation(provincePatch, wardPatch);
        }
        if (provincePatch.action() == ProfilePatch.Action.SET
                && wardPatch.action() != ProfilePatch.Action.SET) {
            throw new InvalidProfileLocationException(Map.of(
                    "wardCode", "must be supplied together with provinceCode"));
        }
        if (wardPatch.action() == ProfilePatch.Action.SET
                && provincePatch.action() != ProfilePatch.Action.SET) {
            throw new InvalidProfileLocationException(Map.of(
                    "provinceCode", "must be supplied together with wardCode"));
        }
        if (!locationCatalog.wardBelongsToProvince(provincePatch.value(), wardPatch.value())) {
            if (locationCatalog.province(provincePatch.value()).isEmpty()) {
                throw new InvalidProfileLocationException(Map.of(
                        "provinceCode", "unknown province code"));
            }
            throw new InvalidProfileLocationException(Map.of(
                    "wardCode", "ward code does not belong to province code"));
        }
        return new ResolvedLocation(provincePatch, wardPatch);
    }

    private CurrentUser toCurrentUser(IdentityUserEntity user, java.util.Set<String> roles) {
        return new CurrentUser(user.id(), user.email(), user.fullName(), roles,
                user.phone(), user.dateOfBirth(), user.gender(),
                user.provinceCode(), user.wardCode(), user.addressLine());
    }

    private record ResolvedLocation(ProfilePatch<String> provinceCode, ProfilePatch<String> wardCode) {
    }
}
