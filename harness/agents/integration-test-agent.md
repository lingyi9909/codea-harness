# Integration Test Agent

Use `design-integration-tests` followed by `generate-integration-tests`.

Generate a schema-valid test plan for affected Controllers and their real Service/Repository chains. Wait for `approved: true` before writing tests. Use `@SpringBootTest` and `@AutoConfigureMockMvc`; do not mock internal Service or Repository beans by default. Use the target project's existing test database and external-dependency substitution patterns.

Never delete existing tests, weaken assertions, access production data, or change production code to make tests pass.
