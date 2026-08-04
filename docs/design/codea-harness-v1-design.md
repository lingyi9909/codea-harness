# Codea Harness V1 Design

Status: approved implementation baseline  
Date: 2026-08-04  
Stack: Java + Spring Boot + Maven

## Goal

Provide an agent-executable specification package for B-end Java services that closes two local development loops: Controller-entry integration testing and local-service log debugging.

## Architecture

The package contains declarative project rules (`AGENTS.md`), executable configuration (`harness.yaml`), four subagent roles, eight reusable skills, nine controlled tool contracts, JSON Schemas for stable outputs, and a validator. V1 does not include a standalone orchestration engine.

## Integration-test path

`MockMvc -> real Controller -> real Service -> real Repository -> existing test database`.

The flow is Git Diff analysis, review, affected Controller/call-chain detection, test-plan generation, approval, test writing, Maven execution, Surefire/log reading, diagnosis, approved minimal production fix, and focused rerun.

## Local-service path

The Harness starts the configured process, records its PID, captures stdout/stderr, checks readiness, records the debugging time window, reads relevant logs after a developer or frontend triggers requests, diagnoses failures, and applies only approved minimal fixes. It stops only the PID it started.

## Roles

- Reviewer: analyze only; produce evidence-backed findings.
- Integration Test Agent: plan first, write after approval, use real internal beans.
- Runtime Debugger: run tests or local service, collect reports/logs, classify failures.
- Fix Agent: propose minimal fixes, modify only after approval, then reverify.

## Out of scope

Task state machines, recovery, multi-service orchestration, full API automation, frontend automation, automatic database/middleware setup, automatic Git publishing, and execution outside a developer machine.
