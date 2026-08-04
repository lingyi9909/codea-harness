# Codea Harness V1 Implementation Plan

**Goal:** Deliver a validated specification package implementing the approved V1 design.

**Architecture:** Declarative Markdown and YAML define behavior; JSON Schemas stabilize outputs; a Python validator checks package completeness, YAML safety constraints, and schemas.

**Tech Stack:** Markdown, YAML 1.2, JSON Schema Draft 2020-12, Python 3.10+, PyYAML, jsonschema.

## Tasks

1. Add repository overview, project instructions, approved design, and example configuration.
2. Add four output schemas: review, test plan, diagnosis, and fix plan.
3. Add four agent definitions and eight skill definitions.
4. Add controlled tool contracts.
5. Add package validator and automated tests.
6. Run all validation and inspect repository status.
