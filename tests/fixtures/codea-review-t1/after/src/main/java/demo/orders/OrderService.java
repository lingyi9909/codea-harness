package demo.orders;

public final class OrderService {
    private final OrderMapper mapper;

    public OrderService(OrderMapper mapper) {
        this.mapper = mapper;
    }

    public void charge(String requestId, String tenantId, String orderId, long cents) {
        mapper.decreaseBalance(tenantId, orderId, cents);
    }
}
