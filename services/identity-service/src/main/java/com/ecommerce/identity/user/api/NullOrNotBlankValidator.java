package com.ecommerce.identity.user.api;

import jakarta.validation.ConstraintValidator;
import jakarta.validation.ConstraintValidatorContext;
import org.springframework.stereotype.Component;

@Component
public class NullOrNotBlankValidator implements ConstraintValidator<NullOrNotBlank, String> {
    @Override
    public boolean isValid(String value, ConstraintValidatorContext context) {
        return value == null || !value.isBlank();
    }
}
