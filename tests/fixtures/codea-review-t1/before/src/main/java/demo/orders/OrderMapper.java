package demo.orders;

public interface OrderMapper {
    void decreaseBalance(String tenantId, String orderId, long cents);
}
