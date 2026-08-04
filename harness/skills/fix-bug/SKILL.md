# Fix Approved Bug

Require an approved `fix-plan.schema.json`. Apply only the listed minimal production changes through `apply_approved_patch`, constrained by allowed and denied paths. Reverify according to the originating path. Never commit, push, create a PR, weaken tests, or perform unrelated refactoring.
