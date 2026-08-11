package com.ecommerce.identity.security;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

import java.io.IOException;
import java.util.Arrays;
import java.util.Collections;
import java.util.Set;
import java.util.UUID;
import java.util.stream.Collectors;

import lombok.extern.slf4j.Slf4j;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

@Slf4j
public class CallerContextFilter extends OncePerRequestFilter {
    static final String USER_ID_HEADER = "X-User-Id";
    static final String ROLES_HEADER = "X-User-Roles";

    private static Set<String> parseRoles(String header) {
        if (header == null || header.isBlank()) {
            return Set.of();
        }
        return Arrays.stream(header.split(","))
                .map(String::trim)
                .filter(value -> !value.isEmpty())
                .collect(Collectors.toUnmodifiableSet());
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain filterChain)
            throws ServletException, IOException {
        String userId = request.getHeader(USER_ID_HEADER);
//        log.info(
//                "request method={} uri={} query={} remote_addr={} content_type={}",
//                request.getMethod(),
//                request.getRequestURI(),
//                request.getQueryString(),
//                request.getRemoteAddr(),
//                request.getContentType()
//        );
//        Collections.list(request.getHeaderNames())
//                .forEach(name ->
//                        log.info("request_header {}={}", name, request.getHeader(name)));
        if (userId != null && !userId.isBlank()) {
            try {
                UUID parsedUserId = UUID.fromString(userId);
                SecurityContextHolder.getContext().setAuthentication(
                        new CallerContextPrincipal(parsedUserId, parseRoles(request.getHeader(ROLES_HEADER))));
            } catch (IllegalArgumentException ignored) {
                // malformed header: stay unauthenticated, authorization decides
            }
        }
        try {
            filterChain.doFilter(request, response);
        } finally {
            SecurityContextHolder.clearContext();
        }
    }
}
