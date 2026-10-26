package com.ecommerce.identity.common.error;

import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.AuthenticationUnavailableException;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import com.ecommerce.identity.auth.domain.InvalidRefreshTokenException;
import com.ecommerce.identity.location.ProvinceNotFoundException;
import com.ecommerce.identity.address.domain.ShippingAddressNotFoundException;
import com.ecommerce.identity.rbac.domain.InvalidRbacItemException;
import com.ecommerce.identity.rbac.domain.RbacHierarchyConflictException;
import com.ecommerce.identity.rbac.domain.RbacItemExistsException;
import com.ecommerce.identity.rbac.domain.RbacItemNotFoundException;
import com.ecommerce.identity.rbac.domain.RbacSystemItemProtectedException;
import com.ecommerce.identity.user.domain.InvalidProfileLocationException;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.admin.domain.LastSuperAdminRequiredException;
import com.ecommerce.identity.user.admin.domain.SelfDisableNotAllowedException;
import com.ecommerce.identity.user.admin.domain.SuperAdminAssignmentForbiddenException;
import jakarta.validation.ConstraintViolation;
import jakarta.validation.ConstraintViolationException;
import jakarta.validation.ElementKind;
import jakarta.validation.Path;
import java.lang.reflect.Method;
import java.util.LinkedHashMap;
import java.util.Map;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.MessageSourceResolvable;
import org.springframework.core.MethodParameter;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.util.ClassUtils;
import org.springframework.validation.method.ParameterErrors;
import org.springframework.validation.method.ParameterValidationResult;
import org.springframework.web.HttpMediaTypeNotAcceptableException;
import org.springframework.web.HttpMediaTypeNotSupportedException;
import org.springframework.web.HttpRequestMethodNotSupportedException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.MissingRequestHeaderException;
import org.springframework.web.bind.MissingServletRequestParameterException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.method.annotation.HandlerMethodValidationException;
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException;
import org.springframework.web.servlet.resource.NoResourceFoundException;

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

    @ExceptionHandler(ConstraintViolationException.class)
    ResponseEntity<ApiError> handleConstraintViolation(ConstraintViolationException exception) {
        Map<String, String> errors = new LinkedHashMap<>();
        exception.getConstraintViolations().forEach(violation -> {
            String field = resolveViolationField(violation);
            log.warn("validation_failed field={} message={}", field, violation.getMessage());
            errors.put(field, violation.getMessage());
        });
        return validationError(errors);
    }

    @ExceptionHandler(HandlerMethodValidationException.class)
    ResponseEntity<ApiError> handleMethodValidation(HandlerMethodValidationException exception) {
        Map<String, String> errors = new LinkedHashMap<>();
        for (ParameterValidationResult result : exception.getParameterValidationResults()) {
            if (result instanceof ParameterErrors parameterErrors) {
                parameterErrors.getFieldErrors().forEach(fieldError -> {
                    log.warn("validation_failed field={} message={}",
                            fieldError.getField(), fieldError.getDefaultMessage());
                    errors.put(fieldError.getField(), fieldError.getDefaultMessage());
                });
                continue;
            }
            String field = resolveParameterName(result.getMethodParameter());
            for (MessageSourceResolvable resolvable : result.getResolvableErrors()) {
                String message = resolvable.getDefaultMessage() != null
                        ? resolvable.getDefaultMessage()
                        : "Invalid value";
                log.warn("validation_failed field={} message={}", field, message);
                errors.putIfAbsent(field, message);
            }
        }
        return validationError(errors);
    }

    @ExceptionHandler(MethodArgumentTypeMismatchException.class)
    ResponseEntity<ApiError> handleTypeMismatch(MethodArgumentTypeMismatchException exception) {
        String field = exception.getName();
        log.warn("validation_failed field={} message=Invalid value", field);
        Map<String, String> errors = (field != null && !field.isBlank())
                ? Map.of(field, "Invalid value")
                : null;
        return validationError(errors);
    }

    @ExceptionHandler(MissingServletRequestParameterException.class)
    ResponseEntity<ApiError> handleMissingParameter(MissingServletRequestParameterException exception) {
        String field = exception.getParameterName();
        log.warn("validation_failed field={} message=Parameter is required", field);
        return validationError(Map.of(field, "Parameter is required"));
    }

    @ExceptionHandler(MissingRequestHeaderException.class)
    ResponseEntity<ApiError> handleMissingHeader(MissingRequestHeaderException exception) {
        String header = exception.getHeaderName();
        log.warn("validation_failed header={} message=Header is required", header);
        return validationError(Map.of(header, "Header is required"));
    }

    @ExceptionHandler(HttpRequestMethodNotSupportedException.class)
    ResponseEntity<ApiError> handleMethodNotSupported(HttpRequestMethodNotSupportedException exception) {
        log.warn("method_not_supported method={}", exception.getMethod());
        return error(HttpStatus.METHOD_NOT_ALLOWED, "METHOD_NOT_ALLOWED", "Request method is not supported");
    }

    @ExceptionHandler(HttpMediaTypeNotSupportedException.class)
    ResponseEntity<ApiError> handleMediaTypeNotSupported(HttpMediaTypeNotSupportedException exception) {
        log.warn("media_type_not_supported contentType={}", exception.getContentType());
        return error(HttpStatus.UNSUPPORTED_MEDIA_TYPE, "UNSUPPORTED_MEDIA_TYPE", "Content type is not supported");
    }

    @ExceptionHandler(HttpMediaTypeNotAcceptableException.class)
    ResponseEntity<ApiError> handleMediaTypeNotAcceptable(HttpMediaTypeNotAcceptableException exception) {
        return ResponseEntity.status(HttpStatus.NOT_ACCEPTABLE)
                .contentType(MediaType.APPLICATION_JSON)
                .body(new ApiError("NOT_ACCEPTABLE", "Requested media type is not acceptable",
                        null, TraceIdProvider.current()));
    }

    @ExceptionHandler(NoResourceFoundException.class)
    ResponseEntity<ApiError> handleNoResourceFound(NoResourceFoundException exception) {
        return error(HttpStatus.NOT_FOUND, "NOT_FOUND", "Resource not found");
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

    @ExceptionHandler(ShippingAddressNotFoundException.class)
    ResponseEntity<ApiError> handleShippingAddressNotFound(ShippingAddressNotFoundException exception) {
        return error(HttpStatus.NOT_FOUND, "ADDRESS_NOT_FOUND", "Shipping address not found");
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

    @ExceptionHandler(SelfDisableNotAllowedException.class)
    ResponseEntity<ApiError> handleSelfDisable(SelfDisableNotAllowedException exception) {
        return error(HttpStatus.CONFLICT, "SELF_DISABLE_NOT_ALLOWED", "An administrator cannot disable their own account");
    }

    @ExceptionHandler(LastSuperAdminRequiredException.class)
    ResponseEntity<ApiError> handleLastSuperAdmin(LastSuperAdminRequiredException exception) {
        return error(HttpStatus.CONFLICT, "LAST_SUPER_ADMIN_REQUIRED",
                "At least one active super administrator is required");
    }

    @ExceptionHandler(SuperAdminAssignmentForbiddenException.class)
    ResponseEntity<ApiError> handleSuperAdminAssignmentForbidden(SuperAdminAssignmentForbiddenException exception) {
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

    private ResponseEntity<ApiError> validationError(Map<String, String> errors) {
        Map<String, String> details = (errors == null || errors.isEmpty()) ? null : errors;
        return ResponseEntity.status(HttpStatus.BAD_REQUEST).body(new ApiError(
                "VALIDATION_FAILED", "Request validation failed", details, TraceIdProvider.current()));
    }

    private static String resolveViolationField(ConstraintViolation<?> violation) {
        Path path = violation.getPropertyPath();
        if (path == null) {
            return "request";
        }
        Path.MethodNode methodNode = null;
        Path.ParameterNode parameterNode = null;
        String leaf = null;
        for (Path.Node node : path) {
            if (node.getKind() == ElementKind.METHOD) {
                methodNode = node.as(Path.MethodNode.class);
            } else if (node.getKind() == ElementKind.PARAMETER) {
                parameterNode = node.as(Path.ParameterNode.class);
            } else if (node.getName() != null && !node.getName().isBlank()) {
                leaf = node.getName();
            }
        }
        if (leaf != null) {
            return leaf;
        }
        if (parameterNode != null) {
            if (methodNode != null && violation.getRootBeanClass() != null) {
                Method method = findDeclaredMethod(
                        ClassUtils.getUserClass(violation.getRootBeanClass()),
                        methodNode.getName(),
                        methodNode.getParameterTypes().toArray(Class<?>[]::new));
                if (method != null) {
                    return resolveParameterName(new MethodParameter(method, parameterNode.getParameterIndex()));
                }
            }
            if (parameterNode.getName() != null && !parameterNode.getName().isBlank()) {
                return parameterNode.getName();
            }
        }
        return "request";
    }

    private static Method findDeclaredMethod(Class<?> targetClass, String name, Class<?>[] parameterTypes) {
        for (Class<?> current = targetClass; current != null && current != Object.class; current = current.getSuperclass()) {
            try {
                return current.getDeclaredMethod(name, parameterTypes);
            } catch (NoSuchMethodException ignored) {
                // Continue searching superclass hierarchy.
            }
        }
        return null;
    }

    private static String resolveParameterName(MethodParameter parameter) {
        RequestParam requestParam = parameter.getParameterAnnotation(RequestParam.class);
        if (requestParam != null && !requestParam.name().isBlank()) {
            return requestParam.name();
        }
        if (requestParam != null && !requestParam.value().isBlank()) {
            return requestParam.value();
        }
        PathVariable pathVariable = parameter.getParameterAnnotation(PathVariable.class);
        if (pathVariable != null && !pathVariable.name().isBlank()) {
            return pathVariable.name();
        }
        if (pathVariable != null && !pathVariable.value().isBlank()) {
            return pathVariable.value();
        }
        RequestHeader requestHeader = parameter.getParameterAnnotation(RequestHeader.class);
        if (requestHeader != null && !requestHeader.name().isBlank()) {
            return requestHeader.name();
        }
        if (requestHeader != null && !requestHeader.value().isBlank()) {
            return requestHeader.value();
        }
        String name = parameter.getParameterName();
        return (name != null && !name.isBlank()) ? name : "arg" + parameter.getParameterIndex();
    }
}
