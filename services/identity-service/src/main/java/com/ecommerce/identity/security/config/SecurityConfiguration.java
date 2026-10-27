package com.ecommerce.identity.security.config;

import com.ecommerce.identity.common.error.ApiError;
import com.ecommerce.identity.common.error.TraceIdProvider;
import com.ecommerce.identity.security.context.CallerContextFilter;
import com.ecommerce.identity.security.context.CallerContextPrincipal;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import tools.jackson.databind.ObjectMapper;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.method.configuration.EnableMethodSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.access.intercept.AuthorizationFilter;

@Configuration
@EnableMethodSecurity
public class SecurityConfiguration {
    @Bean
    SecurityFilterChain securityFilterChain(HttpSecurity http, ObjectMapper objectMapper, Environment environment) throws Exception {
        return http
                .csrf(AbstractHttpConfigurer::disable)
                .sessionManagement(session -> session.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .formLogin(AbstractHttpConfigurer::disable)
                .httpBasic(AbstractHttpConfigurer::disable)
                .logout(AbstractHttpConfigurer::disable)
                .addFilterBefore(new CallerContextFilter(), AuthorizationFilter.class)
                .exceptionHandling(exceptionHandling -> exceptionHandling
                        .authenticationEntryPoint((request, response, exception) -> writeUnauthorized(objectMapper, response))
                        .accessDeniedHandler((request, response, exception) -> writeDenied(objectMapper, response)))
                .authorizeHttpRequests(authorize -> authorize
                        .requestMatchers(request -> isManagementActuatorRequest(request, environment)).permitAll()
                        .requestMatchers(HttpMethod.POST, "/api/v1/auth/register").permitAll()
                        .requestMatchers(HttpMethod.POST, "/api/v1/auth/login").permitAll()
                        .requestMatchers(HttpMethod.POST, "/api/v1/auth/refresh", "/api/v1/auth/logout").permitAll()
                        .requestMatchers(HttpMethod.GET, "/.well-known/jwks.json", "/api/v1/auth/.well-known/jwks.json").permitAll()
                        .requestMatchers(HttpMethod.GET, "/api/v1/users/me", "/api/v1/users/me/permissions").authenticated()
                        .requestMatchers(HttpMethod.PATCH, "/api/v1/users/me").authenticated()
                        .requestMatchers("/api/v1/users/me/addresses", "/api/v1/users/me/addresses/**").authenticated()
                        .requestMatchers("/api/v1/admin/users", "/api/v1/admin/users/**", "/api/v1/admin/rbac/**").authenticated()
                        .requestMatchers(HttpMethod.GET, "/api/v1/locations/provinces").authenticated()
                        .requestMatchers(HttpMethod.GET, "/api/v1/locations/provinces/**").authenticated()
                        .anyRequest().denyAll())
                .build();
    }

    private static boolean isManagementActuatorRequest(HttpServletRequest request, Environment environment) {
        if (!HttpMethod.GET.matches(request.getMethod())) {
            return false;
        }
        Integer managementPort = environment.getProperty("local.management.port", Integer.class);
        Integer serverPort = environment.getProperty("local.server.port", Integer.class);
        if (managementPort == null || managementPort.equals(serverPort) || request.getLocalPort() != managementPort) {
            return false;
        }
        String uri = request.getRequestURI();
        return "/actuator/health".equals(uri)
                || uri.startsWith("/actuator/health/")
                || "/actuator/info".equals(uri);
    }

    private static void writeUnauthorized(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        response.setStatus(HttpStatus.UNAUTHORIZED.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        objectMapper.writeValue(response.getOutputStream(),
                new ApiError("UNAUTHORIZED", "Missing or invalid access token", null, TraceIdProvider.current()));
    }

    private static void writeForbidden(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        response.setStatus(HttpStatus.FORBIDDEN.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        objectMapper.writeValue(response.getOutputStream(),
                new ApiError("FORBIDDEN", "Access is denied", null, TraceIdProvider.current()));
    }

    private static void writeDenied(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        if (SecurityContextHolder.getContext().getAuthentication() instanceof CallerContextPrincipal) {
            writeForbidden(objectMapper, response);
        } else {
            writeUnauthorized(objectMapper, response);
        }
    }
}
