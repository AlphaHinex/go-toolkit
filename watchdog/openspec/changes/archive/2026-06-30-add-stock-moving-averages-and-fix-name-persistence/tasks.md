## 1. Stock MA Data and Notification Format

- [x] 1.1 在 `product/stock.go` 增加常用 MA 周期常量与基于历史收盘价的 SMA 计算函数
- [x] 1.2 在股票通知文本生成路径中追加分周期展示行（固定顺序：5/10/20/60/120/250，每行含 min/max/MA/关系）
- [x] 1.3 为历史数据不足场景增加 `N/A` 降级输出，并保持通知流程不中断

## 2. Stock Name Persistence Fix

- [x] 2.1 在 `watchdog.go` 股票持久化逻辑中回写最新非空股票名称到 `persistedStock.Name`
- [x] 2.2 增加防护：当本轮名称为空时保留配置中已有非空名称

## 3. Validation and Tests

- [x] 3.1 新增或更新测试，覆盖 MA 计算与通知展示格式
- [x] 3.2 新增或更新测试，覆盖股票名称写回 YAML 与空值保护场景
- [x] 3.3 运行相关测试并确认不回归现有基金/股票监控行为