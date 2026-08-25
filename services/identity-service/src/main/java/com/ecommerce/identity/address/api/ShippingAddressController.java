package com.ecommerce.identity.address.api;

import com.ecommerce.identity.address.application.CreateShippingAddressCommand;
import com.ecommerce.identity.address.application.ShippingAddressService;
import com.ecommerce.identity.common.api.ApiResponse;
import com.ecommerce.identity.security.CallerContextPrincipal;
import jakarta.validation.Valid;
import java.util.List;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class ShippingAddressController {
    private final ShippingAddressService service;

    @GetMapping("/users/me/addresses")
    @PreAuthorize("@rbac.can(authentication, 'address.view')")
    ApiResponse<List<ShippingAddressResponse>> list(@AuthenticationPrincipal CallerContextPrincipal caller) {
        return new ApiResponse<>(service.list(caller.userId()).stream().map(ShippingAddressResponse::from).toList(),
                "Shipping addresses retrieved.");
    }

    @PostMapping("/users/me/addresses")
    @PreAuthorize("@rbac.can(authentication, 'address.create')")
    ResponseEntity<ApiResponse<ShippingAddressResponse>> create(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @Valid @RequestBody CreateShippingAddressRequest request) {
        CreateShippingAddressCommand command = new CreateShippingAddressCommand(request.recipientName(), request.phone(),
                request.provinceCode(), request.wardCode(), request.addressLine(), request.label());
        return ResponseEntity.status(HttpStatus.CREATED).body(new ApiResponse<>(
                ShippingAddressResponse.from(service.create(caller.userId(), command)),
                "Shipping address created."));
    }

    @PatchMapping("/users/me/addresses/{addressId}")
    @PreAuthorize("@rbac.can(authentication, 'address.update')")
    ApiResponse<ShippingAddressResponse> update(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @PathVariable("addressId") UUID addressId,
            @Valid @RequestBody UpdateShippingAddressRequest request) {
        return new ApiResponse<>(ShippingAddressResponse.from(
                service.update(caller.userId(), addressId, request.command())),
                "Shipping address updated.");
    }

    @DeleteMapping("/users/me/addresses/{addressId}")
    @PreAuthorize("@rbac.can(authentication, 'address.delete')")
    ApiResponse<Void> delete(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @PathVariable("addressId") UUID addressId) {
        service.delete(caller.userId(), addressId);
        return new ApiResponse<>(null, "Shipping address deleted.");
    }

    @PutMapping("/users/me/addresses/{addressId}/default")
    @PreAuthorize("@rbac.can(authentication, 'address.set-default')")
    ApiResponse<ShippingAddressResponse> setDefault(
            @AuthenticationPrincipal CallerContextPrincipal caller,
            @PathVariable("addressId") UUID addressId) {
        return new ApiResponse<>(ShippingAddressResponse.from(
                service.setDefault(caller.userId(), addressId)),
                "Default shipping address updated.");
    }
}
