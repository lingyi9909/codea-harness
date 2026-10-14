package demo.orders;

public final class OrderService {
    private final OrderMapper mapper;

    public OrderService(OrderMapper mapper) {
        this.mapper = mapper;
    }

    public void charge(String requestId, String tenantId, String orderId, long cents) {
        // T1 fixture intentionally introduces a duplicate-charge risk on retries.
        mapper.decreaseBalance(tenantId, orderId, cents);
    }
}
