package com.ecommerce.identity.common.error;

import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.AuthenticationUnavailableException;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import com.ecommerce.identity.auth.domain.InvalidRefreshTokenException;
import com.ecommerce.identity.location.ProvinceNotFoundException;
import com.ecommerce.identity.rbac.domain.InvalidRbacItemException;
import com.ecommerce.identity.rbac.domain.RbacHierarchyConflictException;
import com.ecommerce.identity.rbac.domain.RbacItemExistsException;
import com.ecommerce.identity.rbac.domain.RbacItemNotFoundException;
import com.ecommerce.identity.rbac.domain.RbacSystemItemProtectedException;
import com.ecommerce.identity.user.domain.InvalidProfileLocationException;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import java.util.LinkedHashMap;
import java.util.Map;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@Slf4j
@RestControllerAdvice
public class GlobalExceptionHandler {
    @ExceptionHandler(MethodArgumentNotValidException.class)
    ResponseEntity<ApiError> handleValidationFailure(MethodArgumentNotValidException exception) {
        Map<String, String> errors = new LinkedHashMap<>();
        exception.getBindingResult().getFieldErrors().forEach(error -> {
            log.warn("validation_failed field={} message={}", error.getField(), error.getDefaultMessage());
            errors.put(error.getField(), error.getDefaultMessage());
        });
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).body(new ApiError(
                "VALIDATION_FAILED", "Request validation failed", errors, TraceIdProvider.current()));
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    ResponseEntity<ApiError> handleInvalidRequest(HttpMessageNotReadableException exception) {
        log.warn("Invalid request", exception);
        return error(HttpStatus.BAD_REQUEST, "VALIDATION_FAILED", "Request validation failed");
    }

    @ExceptionHandler(EmailAlreadyRegisteredException.class)
    ResponseEntity<ApiError> handleDuplicateEmail(EmailAlreadyRegisteredException exception) {
        return error(HttpStatus.CONFLICT, "EMAIL_ALREADY_REGISTERED", "Email is already registered");
    }

    @ExceptionHandler(InvalidCredentialsException.class)
    ResponseEntity<ApiError> handleInvalidCredentials(InvalidCredentialsException exception) {
        return error(HttpStatus.UNAUTHORIZED, "INVALID_CREDENTIALS", "Email or password is invalid.");
    }

    @ExceptionHandler(InvalidRefreshTokenException.class)
    ResponseEntity<ApiError> handleInvalidRefreshToken(InvalidRefreshTokenException exception) {
        return error(HttpStatus.UNAUTHORIZED, "INVALID_REFRESH_TOKEN", "Refresh token is invalid or expired");
    }

    @ExceptionHandler(AuthenticationUnavailableException.class)
    ResponseEntity<ApiError> handleAuthenticationUnavailable(AuthenticationUnavailableException exception) {
        return error(HttpStatus.SERVICE_UNAVAILABLE, "AUTHENTICATION_UNAVAILABLE",
                "Authentication is temporarily unavailable.");
    }

    @ExceptionHandler(UserNotFoundException.class)
    ResponseEntity<ApiError> handleUserNotFound(UserNotFoundException exception) {
        return error(HttpStatus.NOT_FOUND, "USER_NOT_FOUND", "User not found");
    }

    @ExceptionHandler(ProvinceNotFoundException.class)
    ResponseEntity<ApiError> handleProvinceNotFound(ProvinceNotFoundException exception) {
        return error(HttpStatus.NOT_FOUND, "NOT_FOUND", "Province not found");
    }

    @ExceptionHandler(InvalidProfileLocationException.class)
    ResponseEntity<ApiError> handleInvalidProfileLocation(InvalidProfileLocationException exception) {
        log.warn("validation_failed location={}", exception.getMessage());
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).body(new ApiError(
                "VALIDATION_FAILED", "Request validation failed", exception.errors(), TraceIdProvider.current()));
    }

    @ExceptionHandler(InvalidRbacItemException.class)
    ResponseEntity<ApiError> handleInvalidRbacItem(InvalidRbacItemException exception) {
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).body(new ApiError(
                "VALIDATION_FAILED", "Request validation failed", exception.errors(), TraceIdProvider.current()));
    }

    @ExceptionHandler(RbacItemNotFoundException.class)
    ResponseEntity<ApiError> handleRbacItemNotFound(RbacItemNotFoundException exception) {
        return error(HttpStatus.NOT_FOUND, "RBAC_ITEM_NOT_FOUND", "RBAC item not found");
    }

    @ExceptionHandler(RbacItemExistsException.class)
    ResponseEntity<ApiError> handleRbacItemExists(RbacItemExistsException exception) {
        return error(HttpStatus.CONFLICT, "RBAC_ITEM_EXISTS", "RBAC item already exists");
    }

    @ExceptionHandler(RbacHierarchyConflictException.class)
    ResponseEntity<ApiError> handleRbacHierarchyConflict(RbacHierarchyConflictException exception) {
        return error(HttpStatus.CONFLICT, "RBAC_HIERARCHY_CONFLICT", "RBAC hierarchy change is invalid");
    }

    @ExceptionHandler(RbacSystemItemProtectedException.class)
    ResponseEntity<ApiError> handleRbacSystemItemProtected(RbacSystemItemProtectedException exception) {
        return error(HttpStatus.CONFLICT, "RBAC_SYSTEM_ITEM_PROTECTED", "RBAC system item is protected");
    }

    @ExceptionHandler(AuthorizationDeniedException.class)
    ResponseEntity<ApiError> handleAuthorizationDenied(AuthorizationDeniedException exception) {
        return error(HttpStatus.FORBIDDEN, "FORBIDDEN", "Access is denied");
    }

    @ExceptionHandler(Exception.class)
    ResponseEntity<ApiError> handleUnexpectedFailure(Exception exception) {
        log.error("unexpected_request_failure", exception);
        return error(HttpStatus.INTERNAL_SERVER_ERROR, "INTERNAL_ERROR", "An unexpected error occurred");
    }

    private ResponseEntity<ApiError> error(HttpStatus status, String code, String message) {
        return ResponseEntity.status(status).body(new ApiError(code, message, null, TraceIdProvider.current()));
    }
}
