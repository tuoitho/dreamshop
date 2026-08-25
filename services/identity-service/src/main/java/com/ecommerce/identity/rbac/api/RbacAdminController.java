package com.ecommerce.identity.rbac.api;

import com.ecommerce.identity.common.api.ApiResponse;
import com.ecommerce.identity.rbac.application.RbacAdminService;
import com.ecommerce.identity.rbac.domain.AuthItemType;
import jakarta.validation.Valid;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/admin/rbac/items")
@RequiredArgsConstructor
public class RbacAdminController {
    private final RbacAdminService service;

    @GetMapping
    @PreAuthorize("@rbac.can(authentication, 'rbac.view')")
    ApiResponse<List<RbacItemResponse>> list(@RequestParam(value = "type", required = false) AuthItemType type) {
        return new ApiResponse<>(service.list(type).stream().map(RbacItemResponse::from).toList(),
                "RBAC items retrieved.");
    }

    @PostMapping
    @PreAuthorize("@rbac.can(authentication, 'rbac.manage')")
    ResponseEntity<ApiResponse<RbacItemResponse>> create(@Valid @RequestBody CreateRbacItemRequest request) {
        return ResponseEntity.status(HttpStatus.CREATED).body(new ApiResponse<>(
                RbacItemResponse.from(service.create(request.name(), request.type(), request.description())),
                "RBAC item created."));
    }

    @GetMapping("/{name}")
    @PreAuthorize("@rbac.can(authentication, 'rbac.view')")
    ApiResponse<RbacItemResponse> get(@PathVariable("name") String name) {
        return new ApiResponse<>(RbacItemResponse.from(service.get(name)), "RBAC item retrieved.");
    }

    @PatchMapping("/{name}")
    @PreAuthorize("@rbac.can(authentication, 'rbac.manage')")
    ApiResponse<RbacItemResponse> update(@PathVariable("name") String name,
            @Valid @RequestBody UpdateRbacItemRequest request) {
        return new ApiResponse<>(RbacItemResponse.from(service.update(name, request.name(), request.description())),
                "RBAC item updated.");
    }

    @DeleteMapping("/{name}")
    @PreAuthorize("@rbac.can(authentication, 'rbac.manage')")
    ApiResponse<Void> delete(@PathVariable("name") String name) {
        service.delete(name);
        return new ApiResponse<>(null, "RBAC item deleted.");
    }

    @PutMapping("/{parent}/children/{child}")
    @PreAuthorize("@rbac.can(authentication, 'rbac.manage')")
    ApiResponse<Void> addChild(@PathVariable("parent") String parent, @PathVariable("child") String child) {
        service.addChild(parent, child);
        return new ApiResponse<>(null, "RBAC hierarchy updated.");
    }

    @DeleteMapping("/{parent}/children/{child}")
    @PreAuthorize("@rbac.can(authentication, 'rbac.manage')")
    ApiResponse<Void> removeChild(@PathVariable("parent") String parent, @PathVariable("child") String child) {
        service.removeChild(parent, child);
        return new ApiResponse<>(null, "RBAC hierarchy updated.");
    }
}
