# Stock Alert Period Lines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stock single-line MA output with stock-focused period lines (5/10/20/60/120/250), each line showing min/max/MA/relation, while preserving resilient name persistence.

**Architecture:** Keep computation and rendering separate inside product/stock.go. Add a period stats data structure and formatter for per-period lines, then wire PrettyPrint to use it. Preserve existing orchestration in watchdog.go and keep persistence logic guarded by non-empty name overwrite.

**Tech Stack:** Go 1.x, standard library, existing watchdog product/service packages, go test

## Global Constraints

- MA 展示从单行改为按周期逐行展示。
- 默认周期固定为 5、10、20、60、120、250 日。
- 每行必须展示：最小值、最大值、MA 均值、当前价与 MA 关系（高于/低于/持平）。
- 任一周期历史数据不足时，仅该周期降级为 `N/A`，不得中断整条通知。
- 不修改股票筛选与触发阈值逻辑。
- 不引入新配置项和新技术指标。
- 名称持久化规则保持：新值非空覆盖，空值保留旧值。

---

## File Structure

- Modify: `product/stock.go`
  - Add period constants (5/10/20/60/120/250).
  - Add stock period stats model and computation methods.
  - Replace single-line MA formatter with period-line formatter.
  - Keep existing market data retrieval behavior unchanged.
- Modify: `product/stock_test.go`
  - Replace/extend MA tests to validate per-period min/max/MA/relation/N/A rendering.
  - Keep deterministic tests fully offline with synthetic history values.
- Modify: `watchdog.go`
  - Keep current `updatePersistedStock` integration and add explicit assertions through tests only if behavior changes.
- Modify: `watchdog_test.go`
  - Keep name persistence tests and ensure they still pass with new stock output behavior.

### Task 1: Build Period Stats Computation in Stock Domain

**Files:**
- Modify: `product/stock.go`
- Test: `product/stock_test.go`

**Interfaces:**
- Consumes: `func (s *Stock) QueryHistoryValues() []analysis.HistoryValue`
- Produces:
  - `type StockPeriodStats struct { Period int; Min float64; Max float64; MA float64; Available bool }`
  - `func (s *Stock) GetPeriodStats(periods []int) map[int]StockPeriodStats`

- [ ] **Step 1: Write the failing test for period stats calculation**

```go
func TestStock_GetPeriodStats(t *testing.T) {
	now := time.Now()
	s := &Stock{Price: 11}
	for i := 0; i < 10; i++ {
		s.HistoryValues = append(s.HistoryValues, analysis.HistoryValue{
			Date:  now.AddDate(0, 0, -10+i),
			Value: float64(i + 1),
		})
	}

	stats := s.GetPeriodStats([]int{5, 10, 20})
	if !stats[5].Available || stats[5].Min != 6 || stats[5].Max != 10 || stats[5].MA != 8 {
		t.Fatalf("unexpected 5-day stats: %+v", stats[5])
	}
	if !stats[10].Available || stats[10].Min != 1 || stats[10].Max != 10 || stats[10].MA != 5.5 {
		t.Fatalf("unexpected 10-day stats: %+v", stats[10])
	}
	if stats[20].Available {
		t.Fatalf("expected 20-day stats unavailable: %+v", stats[20])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./product -run TestStock_GetPeriodStats -v`
Expected: FAIL with `GetPeriodStats undefined` or assertion failure.

- [ ] **Step 3: Write minimal implementation in stock.go**

```go
type StockPeriodStats struct {
	Period    int
	Min       float64
	Max       float64
	MA        float64
	Available bool
}

func (s *Stock) GetPeriodStats(periods []int) map[int]StockPeriodStats {
	result := make(map[int]StockPeriodStats, len(periods))
	history := s.QueryHistoryValues()

	for _, period := range periods {
		stats := StockPeriodStats{Period: period, Available: false}
		if period <= 0 || len(history) < period {
			result[period] = stats
			continue
		}
		window := history[len(history)-period:]
		minV := window[0].Value
		maxV := window[0].Value
		sum := 0.0
		for _, v := range window {
			if v.Value < minV {
				minV = v.Value
			}
			if v.Value > maxV {
				maxV = v.Value
			}
			sum += v.Value
		}
		stats.Min = minV
		stats.Max = maxV
		stats.MA = sum / float64(period)
		stats.Available = true
		result[period] = stats
	}
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./product -run TestStock_GetPeriodStats -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add product/stock.go product/stock_test.go
git commit -m "feat(stock): add period stats computation for stock alerts"
```

### Task 2: Render Per-Period Lines in PrettyPrint

**Files:**
- Modify: `product/stock.go`
- Test: `product/stock_test.go`

**Interfaces:**
- Consumes:
  - `func (s *Stock) GetPeriodStats(periods []int) map[int]StockPeriodStats`
  - `var CommonMovingAveragePeriods = []int{5, 10, 20, 60, 120, 250}`
- Produces:
  - `func (s *Stock) ComposePeriodStatsRows() string`
  - `func relationToMA(price float64, ma float64) string`

- [ ] **Step 1: Write the failing test for period-line rendering**

```go
func TestStock_ComposePeriodStatsRows(t *testing.T) {
	now := time.Now()
	s := &Stock{Price: 10}
	for i := 0; i < 30; i++ {
		s.HistoryValues = append(s.HistoryValues, analysis.HistoryValue{
			Date:  now.AddDate(0, 0, -30+i),
			Value: 9,
		})
	}

	rows := s.ComposePeriodStatsRows()
	if !strings.Contains(rows, "5日：") || !strings.Contains(rows, "10日：") || !strings.Contains(rows, "20日：") {
		t.Fatalf("missing required period rows: %s", rows)
	}
	if !strings.Contains(rows, "[9.0000, 9.0000]") || !strings.Contains(rows, "MA=9.0000") {
		t.Fatalf("missing min/max/MA format: %s", rows)
	}
	if !strings.Contains(rows, "高于MA") {
		t.Fatalf("missing relation text: %s", rows)
	}
	if !strings.Contains(rows, "120日：N/A") || !strings.Contains(rows, "250日：N/A") {
		t.Fatalf("missing unavailable period markers: %s", rows)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./product -run TestStock_ComposePeriodStatsRows -v`
Expected: FAIL with `ComposePeriodStatsRows undefined` or formatting mismatch.

- [ ] **Step 3: Implement formatter and wire PrettyPrint**

```go
var CommonMovingAveragePeriods = []int{5, 10, 20, 60, 120, 250}

func relationToMA(price float64, ma float64) string {
	if price > ma {
		return "高于MA"
	}
	if price < ma {
		return "低于MA"
	}
	return "持平MA"
}

func (s *Stock) ComposePeriodStatsRows() string {
	stats := s.GetPeriodStats(CommonMovingAveragePeriods)
	lines := make([]string, 0, len(CommonMovingAveragePeriods))
	for _, period := range CommonMovingAveragePeriods {
		entry := stats[period]
		if !entry.Available {
			lines = append(lines, fmt.Sprintf("%d日：N/A", period))
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"%d日：[%.4f, %.4f] MA=%.4f 当前价%s",
			period, entry.Min, entry.Max, entry.MA, relationToMA(s.Price, entry.MA),
		))
	}
	return strings.Join(lines, "\n")
}

func (s *Stock) PrettyPrint() string {
	row := fmt.Sprintf("%s|%s\n", s.Code, s.Name)
	// ... keep existing trend and threshold lines unchanged
	row += s.ComposePeriodStatsRows() + "\n"
	return row + "\n"
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./product -run 'TestStock_ComposePeriodStatsRows|TestStock_GetPeriodStats' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add product/stock.go product/stock_test.go
git commit -m "feat(stock): render per-period min max ma lines in alerts"
```

### Task 3: Preserve Resilient Name Persistence and Update Assertions

**Files:**
- Modify: `watchdog.go`
- Modify: `watchdog_test.go`

**Interfaces:**
- Consumes: `func updatePersistedStock(persistedStock *product.Stock, latest *product.Stock)`
- Produces:
  - `func updatePersistedStock(persistedStock *product.Stock, latest *product.Stock)` (unchanged signature, explicit behavior verification)

- [ ] **Step 1: Write a failing test for non-empty overwrite and empty preserve**

```go
func TestUpdatePersistedStock_NameRules(t *testing.T) {
	persisted := &product.Stock{Name: "旧名称", Price: 1}
	latestNonEmpty := &product.Stock{Name: "新名称", Price: 2}
	updatePersistedStock(persisted, latestNonEmpty)
	if persisted.Name != "新名称" {
		t.Fatalf("expected overwrite with non-empty name, got %q", persisted.Name)
	}

	latestEmpty := &product.Stock{Name: " ", Price: 3}
	updatePersistedStock(persisted, latestEmpty)
	if persisted.Name != "新名称" {
		t.Fatalf("expected preserve on empty name, got %q", persisted.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails (if behavior regressed)**

Run: `go test ./ -run TestUpdatePersistedStock_NameRules -v`
Expected: FAIL if implementation diverges; PASS means behavior already correct and test is accepted as regression guard.

- [ ] **Step 3: Keep or minimally adjust implementation for clarity**

```go
func updatePersistedStock(persistedStock *product.Stock, latest *product.Stock) {
	if strings.TrimSpace(latest.Name) != "" {
		persistedStock.Name = latest.Name
	}
	persistedStock.Price = latest.Price
	persistedStock.LastDayPrice = latest.LastDayPrice
	persistedStock.Datetime = latest.Datetime
	persistedStock.Streak = latest.Streak
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./ -run 'TestUpdatePersistedStock_NameRules|TestUpdatePersistedStock_NameUpdatedWhenLatestNonEmpty|TestUpdatePersistedStock_NamePreservedWhenLatestEmpty' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add watchdog.go watchdog_test.go
git commit -m "test(watchdog): lock stock name persistence behavior"
```

### Task 4: Final Verification, Docs Sync, and Quality Gate

**Files:**
- Modify: `openspec/changes/add-stock-moving-averages-and-fix-name-persistence/tasks.md`
- Modify: `openspec/changes/add-stock-moving-averages-and-fix-name-persistence/specs/stock-alert-enhancements/spec.md` (only if wording must match final output exactly)
- Modify: `docs/superpowers/specs/2026-06-30-stock-alert-period-lines-design.md` (only if implementation wording drifted)

**Interfaces:**
- Consumes:
  - `func (s *Stock) GetPeriodStats(periods []int) map[int]StockPeriodStats`
  - `func (s *Stock) ComposePeriodStatsRows() string`
- Produces:
  - Updated task checklist and validated implementation evidence.

- [ ] **Step 1: Add/update deterministic tests for unavailable period rendering and ordering**

```go
func TestStock_ComposePeriodStatsRows_Order(t *testing.T) {
	s := &Stock{Price: 10}
	rows := s.ComposePeriodStatsRows()
	i5 := strings.Index(rows, "5日：")
	i10 := strings.Index(rows, "10日：")
	i20 := strings.Index(rows, "20日：")
	i60 := strings.Index(rows, "60日：")
	i120 := strings.Index(rows, "120日：")
	i250 := strings.Index(rows, "250日：")
	if !(i5 < i10 && i10 < i20 && i20 < i60 && i60 < i120 && i120 < i250) {
		t.Fatalf("unexpected period order: %s", rows)
	}
}
```

- [ ] **Step 2: Run project checks for changed scope**

Run: `go test ./product -run 'TestStock_GetPeriodStats|TestStock_ComposePeriodStatsRows|TestStock_ComposePeriodStatsRows_Order' -v`
Expected: PASS.

Run: `go test ./ -run 'TestUpdatePersistedStock_NameRules|TestUpdatePersistedStock_NameUpdatedWhenLatestNonEmpty|TestUpdatePersistedStock_NamePreservedWhenLatestEmpty' -v`
Expected: PASS.

- [ ] **Step 3: Validate OpenSpec change metadata**

Run: `openspec validate add-stock-moving-averages-and-fix-name-persistence`
Expected: `Change 'add-stock-moving-averages-and-fix-name-persistence' is valid`.

- [ ] **Step 4: Update docs/checklist to reflect final scope**

```markdown
- Ensure tasks.md checkboxes reflect finished implementation.
- Ensure spec wording says stock periods are 5/10/20/60/120/250 (not 30).
- Ensure design doc examples match per-line format with min/max/MA/relation.
```

- [ ] **Step 5: Commit**

```bash
git add product/stock.go product/stock_test.go watchdog.go watchdog_test.go \
  openspec/changes/add-stock-moving-averages-and-fix-name-persistence/tasks.md \
  openspec/changes/add-stock-moving-averages-and-fix-name-persistence/specs/stock-alert-enhancements/spec.md \
  docs/superpowers/specs/2026-06-30-stock-alert-period-lines-design.md
git commit -m "feat(stock): switch alert MA to per-period lines with resilient fallback"
```

## Self-Review Checklist (Completed)

- Spec coverage: plan tasks cover period-line rendering, min/max/MA, N/A fallback, and name persistence constraints.
- Placeholder scan: no TODO/TBD placeholders; each task includes concrete code and commands.
- Type consistency: `StockPeriodStats`, `GetPeriodStats`, and `ComposePeriodStatsRows` signatures remain consistent across tasks.
