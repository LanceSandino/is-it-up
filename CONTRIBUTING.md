# Contributing

Issues and focused pull requests are welcome. Before opening a pull request:

1. Use Go 1.27.2 or newer and create a branch from `main`.
2. Add or update tests for behavior changes.
3. Run `/bin/bash scripts/test.sh`.
4. Run `/bin/bash scripts/validate-examples.sh`,
   `/bin/bash scripts/static-analysis.sh`, and
   `/bin/bash scripts/security-scan.sh`.
5. Update `CHANGELOG.md` for user-visible changes.

Do not commit credentials, private endpoints, production configuration, or
generated release artifacts. Keep changes small and explain any compatibility
impact in the pull request.
