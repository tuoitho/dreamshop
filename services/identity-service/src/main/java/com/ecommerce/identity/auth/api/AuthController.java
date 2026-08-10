package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.RegisterCommand;
import com.ecommerce.identity.auth.application.RegisterUserService;
import com.ecommerce.identity.auth.application.LoginCommand;
import com.ecommerce.identity.auth.application.LoginService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping
@RequiredArgsConstructor
public class AuthController {
    private final RegisterUserService registerUserService;
    private final LoginService loginService;

    @PostMapping("/register")
    @ResponseStatus(HttpStatus.CREATED)
    RegisterResponse register(@Valid @RequestBody RegisterRequest request) {
        return RegisterResponse.from(registerUserService.register(
                new RegisterCommand(request.email(), request.password(), request.fullName())));
    }

    @PostMapping("/login")
    LoginResponse login(@Valid @RequestBody LoginRequest request) {
        return LoginResponse.from(loginService.login(new LoginCommand(request.email(), request.password())));
    }
}
