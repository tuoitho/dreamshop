package com.ecommerce.identity.common.error;

import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.AuthenticationUnavailableException;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@RestControllerAdvice
public class GlobalExceptionHandler {
    @ExceptionHandler({MethodArgumentNotValidException.class, HttpMessageNotReadableException.class})
    ResponseEntity<ApiError> handleInvalidRequest(Exception exception) {
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

    @ExceptionHandler(AuthenticationUnavailableException.class)
    ResponseEntity<ApiError> handleAuthenticationUnavailable(AuthenticationUnavailableException exception) {
        return error(HttpStatus.SERVICE_UNAVAILABLE, "AUTHENTICATION_UNAVAILABLE",
                "Authentication is temporarily unavailable.");
    }

    @ExceptionHandler(UserNotFoundException.class)
    ResponseEntity<ApiError> handleUserNotFound(UserNotFoundException exception) {
        return error(HttpStatus.NOT_FOUND, "USER_NOT_FOUND", "User not found");
    }

    @ExceptionHandler(Exception.class)
    ResponseEntity<ApiError> handleUnexpectedFailure(Exception exception) {
        return error(HttpStatus.INTERNAL_SERVER_ERROR, "INTERNAL_ERROR", "An unexpected error occurred");
    }

    private ResponseEntity<ApiError> error(HttpStatus status, String code, String message) {
        return ResponseEntity.status(status).body(new ApiError(code, message, TraceIdProvider.current()));
    }
}
