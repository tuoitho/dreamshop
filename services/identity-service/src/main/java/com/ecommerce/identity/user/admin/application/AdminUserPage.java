package com.ecommerce.identity.user.admin.application;

import java.util.List;

public record AdminUserPage(
        List<AdminUserView> items,
        int page,
        int pageSize,
        long totalItems,
        int totalPages,
        long activeCount,
        long disabledCount) {
}
