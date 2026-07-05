## Purpose

Define stock alert enhancements for period-based moving-average statistics and reliable stock name persistence.

## Requirements

### Requirement: Stock Alert SHALL include common moving averages
系统在股票触发通知时 MUST 提供常用周期统计数据，周期至少包含 5、10、20、60、120、250 日，并以逐行方式展示每周期最小值、最大值、MA 均值及当前价格相对关系（高于/低于/持平）。

#### Scenario: Threshold-triggered alert includes MA values
- **WHEN** 某股票满足当前通知触发条件并生成通知文本
- **THEN** 通知中 MUST 包含 5日、10日、20日、60日、120日、250日 的逐行展示项
- **THEN** 每个展示项 MUST 包含该周期最小值、最大值、MA 数值与“高于/低于/持平”状态

#### Scenario: Insufficient history for long period MA
- **WHEN** 股票历史收盘数据不足以计算某个 MA 周期
- **THEN** 通知中 MUST 对该周期输出明确的不可用标记（如 `N/A`）
- **THEN** 系统 MUST 继续输出其余可计算的 MA，不得因单个周期失败中断通知

### Requirement: Stock name MUST be persisted to YAML reliably
系统在写回配置时 MUST 正确持久化股票名称，且在名称获取失败时不得覆盖已有非空名称。

#### Scenario: Persist latest valid stock name
- **WHEN** 本轮行情拉取返回股票名称且名称非空
- **THEN** `stocks.<code>.name` MUST 被更新为最新名称并写入 YAML

#### Scenario: Preserve existing name on fetch failure
- **WHEN** 本轮行情拉取未返回有效名称或返回空名称
- **THEN** 若配置中已有 `stocks.<code>.name` 非空值，系统 MUST 保留该值
- **THEN** 系统 MUST NOT 将该字段写为零值或空字符串
