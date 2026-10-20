package demo.orders;

import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

public final class OrderService {
    private final OrderMapper mapper;
    // A minimal in-process duplicate-request guard; not a distributed idempotency solution.
    private final Set<String> processed = ConcurrentHashMap.newKeySet();

    public OrderService(OrderMapper mapper) {
        this.mapper = mapper;
    }

    public void charge(String requestId, String tenantId, String orderId, long cents) {
        if (!processed.add(requestId)) {
            return;
        }
        mapper.decreaseBalance(tenantId, orderId, cents);
    }
}
