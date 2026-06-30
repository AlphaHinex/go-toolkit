# Verification Warning Fix Brief

Repository: /Users/alphahinex/github/origin/go-toolkit/watchdog

## Goal

Clear the remaining verification warnings after implementing stock alert period lines.

## Required fixes

1. Update OpenSpec artifacts:
   - `openspec/changes/add-stock-moving-averages-and-fix-name-persistence/proposal.md`
   - `openspec/changes/add-stock-moving-averages-and-fix-name-persistence/design.md`

   They must match the final accepted behavior:
   - Stock periods are exactly `5/10/20/60/120/250`.
   - Stock alert output is per-period lines, not one compact single MA line.
   - Each period line includes min, max, MA, and current price relation.
   - Insufficient history for a period prints `N/A` for that period only.
   - Stock name persistence keeps the rule: non-empty latest name overwrites, empty/whitespace latest name preserves existing YAML name.

2. Stabilize full Go test suite without changing runtime stock selection or threshold logic:
   - `go test ./...` currently fails because `product/stock_test.go` has live-data-dependent assertions.
   - `TestStockFactory_GetAllCodes` currently expects exact count `5186`; live API returned `5203`. Make it assert a sensible lower bound and keep suffix validation.
   - `TestStockFactory_SiftIn` currently fails when current live market data does not satisfy the sift criteria. Make this test not fail solely because live data has no signal today. Prefer `t.Skip` with a clear explanation when live data returns no sift result.

## Constraints

- Do not change runtime stock selection or alert trigger logic.
- Do not introduce new config knobs.
- Do not remove deterministic tests added for `GetPeriodStats`, `ComposePeriodStatsRows`, and name persistence.
- Run and report:
  - `go test ./...`
  - `openspec validate add-stock-moving-averages-and-fix-name-persistence` using Node 22 via nvm as needed.

## Report contract

Return:
- Status: DONE / DONE_WITH_CONCERNS / BLOCKED
- Files changed
- Summary of edits
- Test commands and results
- Commit hash if committed
