package com.acme.order;

public class OrderService {
    private final OrderMapper orderMapper = new OrderMapper();

    public String createOrder() {
        return orderMapper.insertOrder();
    }
}
