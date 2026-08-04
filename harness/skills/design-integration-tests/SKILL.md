---
name: design-integration-tests
description: Design Controller-entry Spring Boot integration tests. Map affected Controllers to real Service/Repository chains, identify external mocks, and produce a schema-valid test plan for human approval.
version: 1
agent: integration-test-agent
tools:
  - read_code
output_schema: docs/contracts/test-plan.schema.json
---

# Design Integration Tests

## Purpose
Given a change analysis and review findings, design Controller-entry integration tests. Map affected Controllers to their real Service and Repository chains, identify external dependencies to mock, and produce a schema-valid test plan for human approval.

## When to use
- After `harness review` completes and review findings are available
- When `harness test` proceeds to the test planning phase
- When Integration Test Agent receives change analysis from Reviewer

## Do not use when
- No change analysis has been produced — run `analyze-change` first
- No affected Controllers were found in the change
- The test plan has already been approved and tests are already generated

## Inputs
- Change analysis from `analyze-change` (validated against `change-analysis.schema.json`)
- Review findings from `review-code` (especially items with `needsTest: true`)
- Target project's existing test configuration and external-mock patterns

## Allowed tools
- `read_code` — read existing test files to understand project conventions
- `read_code` — read Controller, Service, Repository source to trace call chains

## Execution steps

1. **Identify affected Controllers**: from the change analysis, list every Controller with changed endpoints or whose downstream Service/Repository chain is modified.
2. **Trace the real call chain**: for each affected endpoint, read the Controller method and trace:
   - Which Service methods it calls
   - Which Repository/Mapper methods those Services call
   - Which external systems (RPC, MQ, third-party API, cache) are invoked
3. **Determine mock strategy**: for each external dependency, identify how the project already mocks or substitutes it in tests (e.g., `@MockBean`, `@SpringBootTest` with test config, WireMock, Testcontainers). Do NOT mock internal Service or Repository beans by default.
4. **Design scenarios**: for each endpoint, design at minimum:
   - **Happy path**: valid request, expected 2xx response, correct state transition and database effects
   - **Error case**: invalid input, expected 4xx response with error body
   - **Edge case**: boundary values, empty inputs, duplicate requests (if idempotency is relevant)
5. **Define preconditions**: for each scenario, specify:
   - Required database state (created through Controller requests or existing test data)
   - External mock responses
   - Authentication/tenant context
6. **Define expected results**: for each scenario, specify:
   - HTTP status code
   - Response body assertions (key fields, not exhaustive)
   - Database state assertions (rows inserted/updated/deleted, specific column values)
   - State transitions (from → to)
7. **Generate planId**: assign a unique ID like `test-plan-YYYYMMDD-NNN`.
8. **Present for approval**: the plan is not yet approved. Prompt the user: "请回复：批准 <planId>".

## Output
Must validate against `docs/contracts/test-plan.schema.json`. Key fields:
- `planId`: unique identifier for this plan
- `targets[]`: per endpoint — controller, endpoint path, serviceChain, repositoryChain, externalMocks, scenarios

## Stop conditions
- No affected Controllers found → report and stop
- Unable to determine mock strategy for an external dependency → flag as a question in the plan, do not guess

## Forbidden actions
- Do not write any test files before the plan is approved
- Do not mock internal Service or Repository beans by default
- Do not design tests that access production data or systems
- Do not weaken assertions to make tests easier to pass

## Example

```json
{
  "planId": "test-plan-20260804-001",
  "targets": [{
    "controller": "OrderController",
    "endpoint": "POST /api/order/approve",
    "serviceChain": ["OrderService"],
    "repositoryChain": ["OrderRepository"],
    "externalMocks": ["OrderRpcClient"],
    "scenarios": [{
      "name": "approve pending order successfully",
      "preconditions": ["a PENDING order exists with id=1"],
      "request": {
        "method": "POST",
        "path": "/api/order/approve",
        "headers": {"Content-Type": "application/json", "X-Tenant-Id": "t1"},
        "body": {"orderId": 1}
      },
      "expected": {
        "httpStatus": 200,
        "responseAssertions": ["$.status == 'APPROVED'"],
        "databaseAssertions": ["order(id=1).status = 'APPROVED'"],
        "stateTransition": {"from": "PENDING", "to": "APPROVED"}
      }
    }]
  }]
}
```
