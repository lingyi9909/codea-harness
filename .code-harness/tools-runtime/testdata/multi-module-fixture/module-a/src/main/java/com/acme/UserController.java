package com.acme;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class UserController {
    private final UserService userService = new UserService();
    @PostMapping("/a/users")
    public String create() {
        return userService.create();
    }
}
