package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.RegisterCommand;
import com.ecommerce.identity.auth.application.RegisterUserService;
import com.ecommerce.identity.auth.application.AuthenticatedSession;
import com.ecommerce.identity.auth.application.LoginCommand;
import com.ecommerce.identity.auth.application.LoginService;
import com.ecommerce.identity.auth.application.RefreshTokenService;
import com.ecommerce.identity.auth.domain.InvalidRefreshTokenException;
import com.ecommerce.identity.common.api.ApiResponse;
import com.ecommerce.identity.common.config.IdentitySessionProperties;
import jakarta.servlet.http.Cookie;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import jakarta.validation.Valid;
import java.time.Duration;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseCookie;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.web.bind.annotation.RestController;

@Slf4j
@RestController
@RequestMapping("/api/v1/auth")
@RequiredArgsConstructor
public class AuthController {
    private final RegisterUserService registerUserService;
    private final LoginService loginService;
    private final RefreshTokenService refreshTokenService;
    private final IdentitySessionProperties properties;

    @PostMapping("/register")
    @ResponseStatus(HttpStatus.CREATED)
    ApiResponse<RegisterResponse> register(@Valid @RequestBody RegisterRequest request) {
        return new ApiResponse<>(RegisterResponse.from(registerUserService.register(
                new RegisterCommand(request.email(), request.password(), request.fullName()))), "Registration successful");
    }

    @PostMapping("/login")
    ApiResponse<LoginResponse> login(@Valid @RequestBody LoginRequest request, HttpServletResponse response) {
        log.info("login_attempt {}", request);
        AuthenticatedSession session = loginService.login(new LoginCommand(request.email(), request.password()));
        response.addHeader(HttpHeaders.SET_COOKIE, cookie(session.rawRefreshToken(), properties.refreshTtl()).toString());
        return new ApiResponse<>(LoginResponse.from(session.accessToken()), "Login successful");
    }

    @PostMapping("/refresh")
    ApiResponse<LoginResponse> refresh(HttpServletRequest request, HttpServletResponse response) {
        try {
            AuthenticatedSession session = refreshTokenService.rotate(refreshToken(request));
            response.addHeader(HttpHeaders.SET_COOKIE,
                    cookie(session.rawRefreshToken(), properties.refreshTtl()).toString());
            return new ApiResponse<>(LoginResponse.from(session.accessToken()), "Token refreshed successfully");
        } catch (InvalidRefreshTokenException exception) {
            response.addHeader(HttpHeaders.SET_COOKIE, cookie("", Duration.ZERO).toString());
            throw exception;
        }
    }

    @PostMapping("/logout")
    ApiResponse<LogoutResponse> logout(HttpServletRequest request, HttpServletResponse response) {
        String rawToken = findRefreshToken(request);
        if (rawToken != null && !rawToken.isBlank()) {
            refreshTokenService.logout(rawToken);
        }
        response.addHeader(HttpHeaders.SET_COOKIE, cookie("", Duration.ZERO).toString());
        return new ApiResponse<>(new LogoutResponse(), "Logout successful");
    }

    private String refreshToken(HttpServletRequest request) {
        String rawToken = findRefreshToken(request);
        if (rawToken == null || rawToken.isBlank()) {
            throw new InvalidRefreshTokenException();
        }
        return rawToken;
    }

    private String findRefreshToken(HttpServletRequest request) {
        Cookie[] cookies = request.getCookies();
        if (cookies == null) {
            return null;
        }
        for (Cookie candidate : cookies) {
            if (properties.cookieName().equals(candidate.getName())) {
                return candidate.getValue();
            }
        }
        return null;
    }

    private ResponseCookie cookie(String value, Duration maxAge) {
        return ResponseCookie.from(properties.cookieName(), value)
                .httpOnly(true)
                .secure(properties.cookieSecure())
                .sameSite("Lax")
                .path(properties.cookiePath())
                .maxAge(maxAge)
                .build();
    }
}
