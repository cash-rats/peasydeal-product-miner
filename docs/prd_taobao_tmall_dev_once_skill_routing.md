# PRD: dev-once 對 Taobao/Tmall 自動觸發 Taobao Skill Set

## 1. 背景
目前 `make dev-once` 在預設情況下走 legacy prompt 路徑，且 skill-mode 實作只允許 Shopee orchestrator。當使用者貼上 Taobao 或 Tmall 商品連結時，無法穩定進入 `taobao-*` skills pipeline。

本 PRD 定義一輪可追蹤改造，目標是讓 `make dev-once` 對 `taobao.com` 與 `tmall.com` 連結都能進入 Taobao skill set。

## 2. 問題陳述
現況阻塞點：
1. `internal/source/source.go` 只接受 `taobao.com`，`tmall.com` 會被判為 unsupported host。
2. `internal/runner/skill_prompt.go` 只允許 Shopee skill-mode，Taobao 會直接報錯。
3. `internal/runner/runner.go` 的 artifact-first fallback 只綁 `shopee-orchestrator-pipeline`。
4. `make dev-once` 未顯式帶 `--prompt-mode=skill`，預設仍走 legacy。

## 3. 目標（Goals）
1. 貼上 `taobao.com` 或 `tmall.com` 商品連結，`make dev-once` 可觸發 Taobao skill pipeline。
2. Shopee 現有 skill-mode 與 artifact-first 行為保持不退化。
3. dev-once 流程維持單指令可用（不需每次手動指定一長串參數）。

## 4. 非目標（Non-Goals）
1. 不重寫 Taobao skill 內容本身（snapshot/core/images/variations/map/orchestrator）。
2. 不引入新 source 類別（Tmall 仍視為 Taobao 生態來源）。
3. 不調整 DB/queue 或 server runtime 行為（僅 devtool/runner 路徑）。

## 5. 使用者故事
1. 作為開發者，我執行 `make dev-once tool=codex url=<taobao_or_tmall_url>`，系統自動走 Taobao orchestrator skill。
2. 作為開發者，我執行 `make dev-once tool=gemini url=<taobao_or_tmall_url>`，也會走同一套 Taobao skill set。
3. 作為開發者，我執行 Shopee URL 時，仍維持既有 Shopee orchestrator skill 行為。

## 6. 方案設計

### 6.1 Source Detection
- 檔案：`internal/source/source.go`
- 調整：把 `tmall.com` 與 `*.tmall.com` 納入 Taobao source 判定。
- 預期：`source.Detect("https://detail.tmall.com/item.htm?..." ) == source.Taobao`。

### 6.2 Skill Name Routing
- 檔案：`internal/runner/skill_prompt.go`
- 調整：
  1. 新增 Taobao 預設 orchestrator skill 常數：`taobao-orchestrator-pipeline`。
  2. `defaultSkillName(source.Taobao)` 回傳 Taobao orchestrator。
  3. `buildSkillPrompt` 不再只限 Shopee，改為依 source 驗證對應 skill 名稱。

建議驗證規則：
- Shopee source 只允許 `shopee-orchestrator-pipeline`。
- Taobao source 只允許 `taobao-orchestrator-pipeline`。

### 6.3 Artifact-first Fallback Generalization
- 檔案：`internal/runner/runner.go`
- 調整：
  1. `isShopeeOrchestratorSkillMode` 改為通用 `isOrchestratorSkillMode`（或等效邏輯）。
  2. `loadOrchestratorFinalResult` 的 skill 檢查改為允許 Shopee/Taobao 兩個 orchestrator skill。
  3. `final.json` path 規則維持不變：`out/artifacts/<run_id>/final.json`。

### 6.4 dev-once 觸發策略
- 檔案：`Makefile`、必要時 `cmd/devtool/cmd/once.go`
- 目標：`make dev-once` 對 Taobao/Tmall 可直接進 skill-mode。

可採策略（本 PRD 建議）：
1. 以環境變數控制為主：
   - `CRAWL_PROMPT_MODE=skill`。
   - `CRAWL_SKILL_NAME` 空值時，由 source 自動選預設 skill。
2. `make dev-once` 不強制覆蓋 `--prompt-mode`，保留 runner 的 env/default 邏輯。
3. README/Makefile help 補充 Taobao/Tmall 用法示例。

## 7. 契約與相容性
1. 最終輸出 contract 仍以現有 crawler contract 為準（`status/title/price/images/...`）。
2. Orchestrator 模式下仍以 `final.json` 作為 artifact-first 來源。
3. 與 Shopee 舊流程相容，不改動既有欄位語意。

## 8. 驗收標準（Acceptance Criteria）

### 8.1 功能驗收
1. `taobao.com` URL：`make dev-once` 可進 Taobao orchestrator skill，並寫出 `out/artifacts/<run_id>/final.json`。
2. `tmall.com` URL：同上，source 能被識別為 Taobao。
3. `shopee.tw` URL：仍走 Shopee orchestrator skill。

### 8.2 錯誤處理
1. skill 缺失時回傳明確錯誤（包含 skill 名稱）。
2. orchestrator `final.json` status=`error` 時，runner 仍回傳 error（既有行為不變）。

### 8.3 回歸
1. `internal/runner/*` 既有 Shopee 測試全部通過。
2. 新增 Taobao/Tmall 路由與 skill prompt 測試。

## 9. 測試計畫

### 9.1 單元測試
1. `internal/source`：新增 `tmall.com` / `detail.tmall.com` 檢測案例。
2. `internal/runner/skill_prompt_test.go`：
   - Taobao source 應產生 `taobao-orchestrator-pipeline` prompt。
   - 不匹配 source/skill 組合應報錯。
3. `internal/runner/runner_orchestrator_fallback_test.go`：
   - Taobao orchestrator skill 可讀 `final.json`。

### 9.2 E2E/手動驗證
1. `make dev-once tool=codex url=<taobao_url>`。
2. `make dev-once tool=codex url=<tmall_url>`。
3. `make dev-once tool=codex url=<shopee_url>`。
4. Gemini tool 重跑至少 1 個 Taobao/Tmall 用例。

## 10. 風險與緩解
1. 風險：將 Tmall 併入 Taobao source 可能影響既有 host 白名單邊界。
   - 緩解：加完整 host 測試，限定 `tmall.com` 與其子網域。
2. 風險：orchestrator fallback 放寬後誤匹配其他 skill。
   - 緩解：明確白名單僅 `shopee-orchestrator-pipeline` / `taobao-orchestrator-pipeline`。
3. 風險：dev-once 預設模式改動造成開發者認知差異。
   - 緩解：README 與 Makefile help 明確示例。

## 11. 里程碑與 Checklist

### 11.1 Routing Core
- [ ] `source.Detect` 支援 `tmall.com` / `*.tmall.com`
- [ ] `defaultSkillName(source.Taobao)` 回傳 `taobao-orchestrator-pipeline`
- [ ] `buildSkillPrompt` 支援 Shopee/Taobao 雙 source 白名單

### 11.2 Runner Fallback
- [ ] orchestrator fallback skill 白名單加入 Taobao
- [ ] `isOrchestratorSkillMode`（或等效）覆蓋 Shopee + Taobao
- [ ] Taobao orchestrator `final.json` 成功讀取測試通過

### 11.3 Dev UX
- [ ] `make dev-once` 使用說明更新（含 Taobao/Tmall）
- [ ] README 補充 skill-mode 設定範例

### 11.4 Validation
- [ ] Taobao URL 實測通過
- [ ] Tmall URL 實測通過
- [ ] Shopee regression 通過

## 12. 成功指標
1. Taobao/Tmall dev-once 進入 skill pipeline 成功率 >= 95%。
2. Shopee dev-once 既有流程成功率不下降。
3. 相關單元測試與 runner 測試通過率 100%。
