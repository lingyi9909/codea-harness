package com.example;

import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class OrderController {
    private final OrderService orderService;

    public OrderController(OrderService orderService) {
        this.orderService = orderService;
    }

    @PostMapping("/orders")
    public void create() {
        orderService.create();
    }

    @PostMapping("/orders/cancel")
    public void cancel() {
        orderService.cancel();
    }
}
