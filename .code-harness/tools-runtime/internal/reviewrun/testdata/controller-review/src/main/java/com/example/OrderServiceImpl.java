package com.example;

import org.springframework.stereotype.Service;

@Service
public class OrderServiceImpl implements OrderService {
    private final OrderMapper orderMapper;

    public OrderServiceImpl(OrderMapper orderMapper) {
        this.orderMapper = orderMapper;
    }

    @Override
    public void create() {
        orderMapper.insertOrder();
    }

    @Override
    public void cancel() {
        orderMapper.cancelOrder();
    }
}
