package com.ecommerce.identity.user.api;

import com.ecommerce.identity.security.CallerContextPrincipal;
import com.ecommerce.identity.common.api.ApiResponse;
import com.ecommerce.identity.user.application.MeService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PatchMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class MeController {
    private final MeService meService;

    @GetMapping("/users/me")
    MeResponse me(@AuthenticationPrincipal CallerContextPrincipal caller) {
        return MeResponse.from(meService.current(caller.userId()));
    }

    @PatchMapping("/users/me")
    ApiResponse<MeUpdateResponse> update(@AuthenticationPrincipal CallerContextPrincipal caller,
            @Valid @RequestBody UpdateMeRequest request) {
        return new ApiResponse<>(MeUpdateResponse.from(meService.update(caller.userId(), request.fullName(), request.phone(),
                request.dateOfBirth(), request.gender(), request.provinceCode(),
                request.wardCode(), request.addressLine())), "Profile updated successfully");
    }
}
