package com.ecommerce.identity.security;

import com.ecommerce.identity.common.error.ApiError;
import com.ecommerce.identity.common.error.TraceIdProvider;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import tools.jackson.databind.ObjectMapper;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.access.intercept.AuthorizationFilter;

@Configuration
public class SecurityConfiguration {
    @Bean
    CallerContextFilter callerContextFilter() {
        return new CallerContextFilter();
    }

    @Bean
    SecurityFilterChain securityFilterChain(HttpSecurity http, CallerContextFilter callerContextFilter,
            ObjectMapper objectMapper) throws Exception {
        return http
                .csrf(AbstractHttpConfigurer::disable)
                .sessionManagement(session -> session.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .formLogin(AbstractHttpConfigurer::disable)
                .httpBasic(AbstractHttpConfigurer::disable)
                .logout(AbstractHttpConfigurer::disable)
                .addFilterBefore(callerContextFilter, AuthorizationFilter.class)
                .exceptionHandling(exceptionHandling -> exceptionHandling
                        .authenticationEntryPoint((request, response, exception) -> writeUnauthorized(objectMapper, response))
                        .accessDeniedHandler((request, response, exception) -> writeDenied(objectMapper, response)))
                .authorizeHttpRequests(authorize -> authorize
                        .requestMatchers(HttpMethod.POST, "/register").permitAll()
                        .requestMatchers(HttpMethod.POST, "/login").permitAll()
                        .requestMatchers(HttpMethod.GET, "/.well-known/jwks.json").permitAll()
                        .requestMatchers(HttpMethod.GET, "/actuator/health", "/actuator/health/**").permitAll()
                        .requestMatchers(HttpMethod.GET, "/users/me").authenticated()
                        .anyRequest().denyAll())
                .build();
    }

    private static void writeUnauthorized(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        response.setStatus(HttpStatus.UNAUTHORIZED.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        objectMapper.writeValue(response.getOutputStream(),
                new ApiError("UNAUTHORIZED", "Missing or invalid access token", TraceIdProvider.current()));
    }

    private static void writeForbidden(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        response.setStatus(HttpStatus.FORBIDDEN.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        objectMapper.writeValue(response.getOutputStream(),
                new ApiError("FORBIDDEN", "Access is denied", TraceIdProvider.current()));
    }

    private static void writeDenied(ObjectMapper objectMapper, HttpServletResponse response) throws IOException {
        if (SecurityContextHolder.getContext().getAuthentication() instanceof CallerContextPrincipal) {
            writeForbidden(objectMapper, response);
        } else {
            writeUnauthorized(objectMapper, response);
        }
    }
}
