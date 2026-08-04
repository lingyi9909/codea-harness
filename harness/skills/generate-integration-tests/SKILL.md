---
name: generate-integration-tests
description: Given a human-approved test plan, generate or modify Spring Boot integration test classes using @SpringBootTest + @AutoConfigureMockMvc with real internal beans.
version: 1
agent: integration-test-agent
tools:
  - read_code
  - write_test
output_schema: null
---

# Generate Integration Tests

## Purpose
Given a human-approved test plan, generate or modify integration test classes using `@SpringBootTest` + `@AutoConfigureMockMvc` with real internal beans and the project's existing test database configuration.

## When to use
- After the user explicitly approves a test plan by its `planId` (e.g., "批准 test-plan-20260804-001")
- When Integration Test Agent proceeds from plan approval to code generation

## Do not use when
- The test plan has not been explicitly approved
- The approval is vague ("ok", "继续") — must contain the exact `planId`
- The plan has been modified since approval — generate a new `planId` first

## Inputs
- An approved test plan (`planId` must have been explicitly approved by a human)
- The target project's existing test directory structure and conventions
- The project's existing `@SpringBootTest` configuration (test database, external mock setup)

## Allowed tools
- `read_code` — read existing test files, test configuration, and source code
- `write_test` — create or modify test files under allowed test paths

## Execution steps

1. **Verify approval**: confirm the test plan identified by `planId` has been explicitly approved by a human with the exact `planId` in the approval message. If not, stop and request approval.
2. **Study conventions**: read 1-2 existing integration test files in the project to understand:
   - Base test class or common annotations
   - Test class naming pattern (e.g., `XxxControllerIT`, `XxxIntegrationTest`)
   - Assertion library (AssertJ, Hamcrest, JUnit assertions)
   - How external dependencies are mocked (`@MockBean`, test configuration, WireMock)
   - How test data is prepared (SQL scripts, `@BeforeEach`, builder methods)
   - Authentication/tenant context setup in tests
3. **Determine test class location**: place new tests under `src/test/java/` mirroring the production package structure. Follow the project's existing naming convention.
4. **Write the test class**:
   - Annotate with `@SpringBootTest` and `@AutoConfigureMockMvc`
   - Use `@Autowired MockMvc` for requests
   - Use `@Autowired` for real Controller, Service, Repository beans when needed for setup/verification
   - Mock only external dependencies (RPC, MQ, third-party APIs) using the project's existing pattern
   - Use the project's existing test database profile/configuration
5. **Implement each scenario** from the plan:
   - **Setup**: prepare preconditions (create data through Controller requests or repository setup)
   - **Execute**: send the request via `MockMvc.perform()`
   - **Assert**: verify HTTP status, response body key fields, and database state changes
   - **Teardown**: rely on `@Transactional` rollback or the project's existing cleanup mechanism
6. **Handle error scenarios**: for 4xx/5xx expected responses, assert the error code/message structure matching the project's unified error response format.
7. **Preserve existing tests**: never delete, disable, or weaken existing test assertions.

## Output
- New or modified test files under `write.allowedTestPaths`
- Each test method corresponds to a scenario in the approved plan

## Stop conditions
- Test plan is not approved → stop and request approval
- Test path is not in allowed paths → stop and report
- Existing test conventions cannot be determined → flag and ask

## Forbidden actions
- Do not write tests without an approved plan
- Do not mock internal Service or Repository beans by default
- Do not delete existing tests, add `@Disabled`, comment out assertions, or weaken assertions
- Do not access production data or systems
- Do not change production code to make tests pass

## Example

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
        // setup: create a pending order
        Order order = orderRepository.save(
            new Order().setStatus(Status.PENDING).setTenantId("t1"));

        // mock external RPC
        when(orderRpcClient.notifyErp(any())).thenReturn(true);

        // execute
        mockMvc.perform(post("/api/order/approve")
                .contentType(MediaType.APPLICATION_JSON)
                .header("X-Tenant-Id", "t1")
                .content("{\"orderId\": " + order.getId() + "}"))
            // assert
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("APPROVED"));

        // verify database
        Order updated = orderRepository.findById(order.getId()).orElseThrow();
        assertThat(updated.getStatus()).isEqualTo(Status.APPROVED);
    }
}
```
