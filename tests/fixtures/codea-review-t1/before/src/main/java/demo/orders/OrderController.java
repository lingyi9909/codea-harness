package demo.orders;

/** This entry point deliberately stays unchanged in the feature branch. */
public final class OrderController {
    private final OrderService service;

    public OrderController(OrderService service) {
        this.service = service;
    }

    public void charge(String requestId, String tenantId, String orderId, long cents) {
        service.charge(requestId, tenantId, orderId, cents);
    }
}
