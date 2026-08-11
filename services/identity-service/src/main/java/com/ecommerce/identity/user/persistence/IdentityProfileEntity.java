package com.ecommerce.identity.user.persistence;

import com.ecommerce.identity.user.domain.Gender;
import com.ecommerce.identity.user.domain.ProfilePatch;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.FetchType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.MapsId;
import jakarta.persistence.OneToOne;
import jakarta.persistence.Table;
import java.time.LocalDate;
import java.util.Objects;
import java.util.UUID;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;

@Entity
@Table(name = "identity_profiles")
@Getter(AccessLevel.PACKAGE)
@Accessors(fluent = true)
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class IdentityProfileEntity {
    @Id
    @Column(name = "user_id")
    private UUID userId;

    @MapsId
    @OneToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "user_id")
    private IdentityUserEntity user;

    @Column(name = "full_name", nullable = false, length = 100)
    private String fullName;

    @Column(length = 20)
    private String phone;

    @Column(name = "date_of_birth")
    private LocalDate dateOfBirth;

    @Enumerated(EnumType.STRING)
    @Column(length = 16)
    private Gender gender;

    @Column(name = "province_code", length = 10)
    private String provinceCode;

    @Column(name = "ward_code", length = 10)
    private String wardCode;

    @Column(name = "address_line", length = 255)
    private String addressLine;

    IdentityProfileEntity(IdentityUserEntity user, String fullName) {
        this.user = Objects.requireNonNull(user, "user");
        this.fullName = Objects.requireNonNull(fullName, "fullName");
    }

    void applyUpdate(ProfilePatch<String> fullName, ProfilePatch<String> phone, ProfilePatch<LocalDate> dateOfBirth,
            ProfilePatch<Gender> gender, ProfilePatch<String> provinceCode, ProfilePatch<String> wardCode,
            ProfilePatch<String> addressLine) {
        this.fullName = fullName.applyTo(this.fullName);
        this.phone = phone.applyTo(this.phone);
        this.dateOfBirth = dateOfBirth.applyTo(this.dateOfBirth);
        this.gender = gender.applyTo(this.gender);
        this.provinceCode = provinceCode.applyTo(this.provinceCode);
        this.wardCode = wardCode.applyTo(this.wardCode);
        this.addressLine = addressLine.applyTo(this.addressLine);
    }
}
