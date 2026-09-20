package com.acme.order;

import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class OrderController {
    private final OrderService orderService = new OrderService();

    @PostMapping("/orders")
    public String createOrder() {
        return orderService.createOrder();
    }
}
