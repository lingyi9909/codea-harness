package com.acme.order;

public class OrderController {
    private final OrderService orderService = new OrderService();

    public String createOrder() {
        return orderService.createOrder();
    }
}
