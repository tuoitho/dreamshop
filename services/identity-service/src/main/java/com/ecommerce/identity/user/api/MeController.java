package com.ecommerce.identity.user.api;

import com.ecommerce.identity.security.CallerContextPrincipal;
import com.ecommerce.identity.user.application.MeService;
import lombok.RequiredArgsConstructor;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class MeController {
    private final MeService meService;

    @GetMapping("/users/me")
    MeResponse me(Authentication authentication) {
        CallerContextPrincipal caller = (CallerContextPrincipal) authentication.getPrincipal();
        return MeResponse.from(meService.current(caller.userId()));
    }
}
