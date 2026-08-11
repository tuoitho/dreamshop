package com.ecommerce.identity.user.api;

import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.Nulls;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;
import lombok.Getter;
import lombok.experimental.Accessors;

@Getter
@Accessors(fluent = true)
public class UpdateMeRequest {
    @NullOrNotBlank
    @Size(max = 100)
    private String fullName;

    @Size(max = 20)
    @Pattern(regexp = "^(?:(?:\\+84|0)(?:3|5|7|8|9)\\d{8})?$",
            message = "must be a valid Vietnamese mobile number")
    private String phone;

    @PastDate
    private String dateOfBirth;

    @Pattern(regexp = "^(?:MALE|FEMALE|OTHER)?$",
            message = "must be one of MALE, FEMALE, OTHER")
    private String gender;

    @Size(max = 10)
    private String provinceCode;

    @Size(max = 10)
    private String wardCode;

    @Size(max = 255)
    private String addressLine;

    @JsonSetter(value = "fullName", nulls = Nulls.FAIL)
    public UpdateMeRequest fullName(String fullName) {
        this.fullName = fullName == null ? null : fullName.trim();
        return this;
    }

    @JsonSetter("phone")
    public UpdateMeRequest phone(String phone) {
        this.phone = phone;
        return this;
    }

    @JsonSetter("dateOfBirth")
    public UpdateMeRequest dateOfBirth(String dateOfBirth) {
        this.dateOfBirth = dateOfBirth;
        return this;
    }

    @JsonSetter("gender")
    public UpdateMeRequest gender(String gender) {
        this.gender = gender;
        return this;
    }

    @JsonSetter("provinceCode")
    public UpdateMeRequest provinceCode(String provinceCode) {
        this.provinceCode = provinceCode;
        return this;
    }

    @JsonSetter("wardCode")
    public UpdateMeRequest wardCode(String wardCode) {
        this.wardCode = wardCode;
        return this;
    }

    @JsonSetter("addressLine")
    public UpdateMeRequest addressLine(String addressLine) {
        this.addressLine = addressLine;
        return this;
    }
}
