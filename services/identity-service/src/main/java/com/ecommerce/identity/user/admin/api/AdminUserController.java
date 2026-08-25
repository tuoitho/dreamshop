package com.ecommerce.identity.user.admin.api;

import com.ecommerce.identity.common.api.ApiResponse;
import com.ecommerce.identity.security.context.CallerContextPrincipal;
import com.ecommerce.identity.user.admin.application.AdminUserService;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@Validated
@RequestMapping("/admin/users")
@RequiredArgsConstructor
public class AdminUserController {
    private final AdminUserService service;

    @GetMapping
    @PreAuthorize("@rbac.can(authentication, 'user.view')")
    ApiResponse<AdminUserPageResponse> list(
            @RequestParam(value = "q", defaultValue = "") String query,
            @RequestParam(value = "page", defaultValue = "1") @Min(1) int page,
            @RequestParam(value = "pageSize", defaultValue = "20") @Min(1) @Max(100) int pageSize) {
        return new ApiResponse<>(AdminUserPageResponse.from(service.list(query, page, pageSize)),
                "Users retrieved.");
    }

    @GetMapping("/{userId}")
    @PreAuthorize("@rbac.can(authentication, 'user.view')")
    ApiResponse<AdminUserResponse> get(@PathVariable("userId") UUID userId) {
        return new ApiResponse<>(AdminUserResponse.from(service.get(userId)), "User retrieved.");
    }

    @PatchMapping("/{userId}/status")
    @PreAuthorize("@rbac.can(authentication, 'user.status.update')")
    ApiResponse<AdminUserResponse> changeStatus(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @PathVariable("userId") UUID userId,
            @Valid @RequestBody UpdateUserStatusRequest request) {
        return new ApiResponse<>(AdminUserResponse.from(service.changeStatus(caller.userId(), userId, request.status())),
                "User status updated.");
    }

    @PutMapping("/{userId}/roles/{roleName}")
    @PreAuthorize("@rbac.can(authentication, 'user.role.assign')")
    ApiResponse<AdminUserResponse> assignRole(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @PathVariable("userId") UUID userId,
            @PathVariable("roleName") String roleName) {
        return new ApiResponse<>(AdminUserResponse.from(service.assignRole(caller.userId(), userId, roleName)),
                "User role assigned.");
    }

    @DeleteMapping("/{userId}/roles/{roleName}")
    @PreAuthorize("@rbac.can(authentication, 'user.role.assign')")
    ApiResponse<AdminUserResponse> revokeRole(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @PathVariable("userId") UUID userId,
            @PathVariable("roleName") String roleName) {
        return new ApiResponse<>(AdminUserResponse.from(service.revokeRole(caller.userId(), userId, roleName)),
                "User role revoked.");
    }
}
