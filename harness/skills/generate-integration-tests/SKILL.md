# Generate Integration Tests

Require a test plan with `approved: true`. Use `write_test` only in allowed test paths. Prefer `@SpringBootTest` plus `@AutoConfigureMockMvc`, real internal beans, existing test database configuration, and project-standard external mocks. Preserve existing tests and assertions.
