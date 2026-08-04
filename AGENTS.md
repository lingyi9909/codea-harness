# Project Instructions for Codea Harness

## Scope

This repository defines Codea Harness V1. Keep changes limited to specification, contracts, agent instructions, skill instructions, example configuration, and package validation.

## V1 behavior

- Review starts from Git Diff and reads only directly related call-chain code.
- Integration tests enter through MockMvc and use real Controller, Service, Repository, and the project's existing test database configuration.
- Internal Service and Repository beans are not mocked by default.
- External systems, third-party APIs, MQ, and RPC follow the target project's existing test substitution method.
- Local service debugging is independent from integration-test execution.

## Required gates

- Do not write or modify test code before an approved test plan exists.
- Do not modify production code before an approved fix plan exists.
- A newly generated failing test may be repaired and rerun without a second approval when the issue is in test code.

## Prohibited behavior

- No arbitrary shell construction by subagents.
- No production database access.
- No automatic dependency environment provisioning.
- No automatic commit, push, or pull-request creation.
- No unrelated refactoring or weakened assertions.
