package com.ecommerce.identity.user.admin.api;

import com.ecommerce.identity.user.admin.application.AdminUserPage;
import java.util.List;

public record AdminUserPageResponse(
        List<AdminUserResponse> items,
        int page,
        int pageSize,
        long totalItems,
        int totalPages) {
    public static AdminUserPageResponse from(AdminUserPage page) {
        return new AdminUserPageResponse(page.items().stream().map(AdminUserResponse::from).toList(),
                page.page(), page.pageSize(), page.totalItems(), page.totalPages());
    }
}
