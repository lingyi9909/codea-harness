---
name: generate-integration-tests
description: 根据已审批的测试计划，生成使用 @SpringBootTest + @AutoConfigureMockMvc 的集成测试类，真实调用内部 Bean。
version: 1
agent: integration-test-agent
tools:
  - read_code
  - write_test
output_schema: null
---

# 生成集成测试

## 目标

根据经人工审批的测试计划，使用 `@SpringBootTest` + `@AutoConfigureMockMvc` 生成或修改集成测试类，真实调用内部 Bean，使用项目已有的测试数据库配置。

## 适用场景

- 用户以精确 `planId` 明确审批测试计划后（如「批准 test-plan-20260804-001」）
- Integration Test Agent 从计划审批阶段进入代码生成阶段

## 不适用场景

- 测试计划尚未被明确审批
- 审批表述模糊（「好」「继续」）——必须包含精确的 `planId`
- 计划自审批以来已被修改——需先生成新的 `planId`

## 输入

- 已审批的测试计划（`planId` 必须已经人工明确审批）
- 目标项目已有的测试目录结构和约定
- 项目已有的 `@SpringBootTest` 配置（测试数据库、外部 Mock 设置）

## 允许使用的工具

- `read_code`——读取已有测试文件、测试配置和源码
- `write_test`——在允许的测试路径下创建或修改测试文件

## 前置条件

- 测试计划已获人工审批
- 目标项目的测试目录结构可访问
- `harness.yaml` 中 `write.allowedTestPaths` 配置有效

## 执行步骤

1. **验证审批**：确认以 `planId` 标识的测试计划已被人工明确审批，审批消息中包含精确的 `planId`。如未审批，停止并请求审批。
2. **学习约定**：阅读项目中 1-2 个已有的集成测试文件，了解：
   - 基类或通用注解
   - 测试类命名规则（如 `XxxControllerIT`、`XxxIntegrationTest`）
   - 断言库（AssertJ、Hamcrest、JUnit assertions）
   - 外部依赖的 Mock 方式（`@MockBean`、测试配置、WireMock）
   - 测试数据准备方式（SQL 脚本、`@BeforeEach`、Builder 方法）
   - 测试中的认证/租户上下文设置
3. **确定测试类位置**：将新测试放在 `src/test/java/` 下，镜像生产包结构。遵循项目已有的命名约定。
4. **编写测试类**：
   - 使用 `@SpringBootTest` 和 `@AutoConfigureMockMvc` 注解
   - 使用 `@Autowired MockMvc` 发送请求
   - 使用 `@Autowired` 注入真实 Controller、Service、Repository Bean（用于 setup/verification）
   - 仅 Mock 外部依赖（RPC、MQ、第三方接口），沿用项目已有方式
   - 使用项目已有的测试数据库 profile/配置
5. **实现每个场景**：
   - **准备**：构造前置条件（通过 Controller 请求或 Repository 操作创建数据）
   - **执行**：通过 `MockMvc.perform()` 发送请求
   - **断言**：验证 HTTP 状态码、响应体关键字段和数据库状态变更
   - **清理**：依赖 `@Transactional` 回滚或项目已有的清理机制
6. **处理错误场景**：对 4xx/5xx 预期响应，断言错误码/消息结构匹配项目统一错误响应格式。
7. **保留已有测试**：绝不删除、禁用或弱化已有的测试断言。

## 输出

- 新建或修改的测试文件（位于 `write.allowedTestPaths` 下）
- 每个测试方法对应审批通过的计划中的一个场景

## 停止条件

- 测试计划未获审批 → 停止并请求审批
- 测试路径不在允许的路径中 → 停止并报告
- 无法确定已有测试约定 → 标记并询问

## 禁止行为

- 不得在计划审批通过前编写测试代码
- 不得默认 Mock 项目内部的 Service 或 Repository Bean
- 不得删除已有测试、添加 `@Disabled`、注释掉断言或弱化断言
- 不得访问生产数据或系统
- 不得为让测试通过而修改生产代码

## 示例

```java
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
@AutoConfigureMockMvc
@ActiveProfiles("test")
class OrderControllerIT {

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private OrderRepository orderRepository;

    @MockBean
    private OrderRpcClient orderRpcClient;

    @Test
    void shouldApprovePendingOrder() throws Exception {
        // 准备：创建待处理订单
        Order order = orderRepository.save(
            new Order().setStatus(Status.PENDING).setTenantId("t1"));

        // Mock 外部 RPC
        when(orderRpcClient.notifyErp(any())).thenReturn(true);

        // 执行
        mockMvc.perform(post("/api/order/approve")
                .contentType(MediaType.APPLICATION_JSON)
                .header("X-Tenant-Id", "t1")
                .content("{\"orderId\": " + order.getId() + "}"))
            // 断言
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("APPROVED"));

        // 验证数据库
        Order updated = orderRepository.findById(order.getId()).orElseThrow();
        assertThat(updated.getStatus()).isEqualTo(Status.APPROVED);
    }
}
```
