# PRD: Taobao Skills 單一能力逐步驗證（Feasibility + Correctness）

## 1. 背景與目標
現有 Shopee skills pipeline 已採分階段設計（snapshot -> core -> images -> variations -> variation-image-map -> orchestrator）。

Taobao 在頁面結構、資料來源、反爬行為、互動流程上與 Shopee 顯著不同，若一次開發整套 skills，除錯成本高且風險聚集。

本 PRD 定義「一次只做一個 skill」的落地策略：
1. 先驗證單一 skill 的可行性。
2. 再驗證單一 skill 的正確性（輸出契約 + 基本品質）。
3. 通過後才進到下一個 skill。

## 2. 問題陳述
- Shopee 專用 parser/selector 無法直接套用到 Taobao。
- Taobao 可能遇到登入牆、驗證碼、滑塊、人機風控，導致資料不可見。
- 直接做 full pipeline 時，很難分辨是哪個 stage 失敗。

## 3. 產品目標（Goals）
- 建立一套 Taobao skills，沿用既有 artifact-first 設計。
- 每個 skill 都有可獨立執行、可重現、可驗收的測試流程。
- 每個 skill 都有明確輸入/輸出契約與錯誤降級策略。

## 4. 非目標（Non-Goals）
- 本 PRD 不要求一次完成 Taobao orchestrator。
- 本 PRD 不處理跨站通用抽象層（僅聚焦 Taobao）。
- 本 PRD 不承諾一次解決所有驗證牆情境（先以 needs_manual 正確降級為主）。

## 5. 範圍
### In Scope
- 新增 Taobao 專用 skill 定義與腳本（逐個交付）。
- 每個 skill 的 artifact、JSON contract、驗收標準。
- 以真實 Taobao 商品頁做 smoke test 與對照驗證。

### Out of Scope
- 大規模壓測與分散式排程整合。
- 站外資料補全（例如第三方價格 API）。

## 6. 交付策略（One Skill at a Time）
執行順序固定如下，未通過驗收不得進下一個：
1. `taobao-page-snapshot`
2. `taobao-product-core`
3. `taobao-product-images`
4. `taobao-product-variations`
5. `taobao-variation-image-map`
6. `taobao-orchestrator-pipeline`（最後整合）

## 7. 每階段通用完成定義（Definition of Done）
每個 skill 必須同時滿足：
1. 可獨立執行（輸入明確，無隱性前置依賴）。
2. 輸出 JSON 嚴格符合契約（欄位完整、型別正確）。
   - Taobao skills 輸出規範與 Shopee skills 完全一致（含欄位命名、狀態值、JSON-only stdout 規則），除非 PRD 明確註記例外。
3. 失敗可預期（`needs_manual` 或 `error`，且錯誤訊息非空）。
4. 產物可落地到 `out/artifacts/<run_id>/`。
5. 具至少 3 組測試樣本：
   - 正常可見商品頁
   - 規格較複雜商品頁
   - 受限頁（登入/驗證牆或內容缺失）

## 8. 階段規格與驗收
## 8.1 Skill 1: `taobao-page-snapshot`
### 目的
建立穩定 HTML snapshot 基礎，支撐後續全離線解析。

### 必要輸出
- `s0-initial.html.gz`
- `s0-overlay.html.gz`（best-effort）
- `s0-variation-<position>.html.gz`（best-effort, up to 20）
- `s0-manifest.json`
- `s0-snapshot-pointer.json`

Phase-1 scope:
- variation snapshots 先只針對 `颜色分类` 群組互動與擷取。
- 若頁面無 `颜色分类`，才 fallback 到第一個可用規格群組，並在 notes 記錄。

### 驗收標準
- 若核心商品區域可見：`status=ok`。
- 若被驗證牆阻擋且核心資訊不可見：`status=needs_manual`，`notes` 非空。
- `tab_tracking` 完整，含 close attempt/success/failure。

## 8.2 Skill 2: `taobao-product-core`
### 目的
離線抽取 `title/description/currency/price`。

### 驗收標準
- `status=ok` 時四欄皆非空。
- `description` 長度上限 1500。
- 無法完整抽取時：`status=error` 且 `error` 非空。
- 若 manifest/html 判斷受阻：`status=needs_manual`。

## 8.3 Skill 3: `taobao-product-images`
### 目的
離線抽取商品圖片 URL 陣列。

### 驗收標準
- 僅允許 `http/https`。
- 去重後最多 20 筆。
- 找不到圖片仍回 `status=ok` + `images=[]`。
- 讀檔/解析致命失敗才可 `status=error`。

## 8.4 Skill 4: `taobao-product-variations`
### 目的
離線抽取規格選項（title/position/price）。

### 驗收標準
- 每筆包含 `title`, `position`, `price`（無價時可空字串）。
- 去重後最多 20 筆。
- 找不到規格仍回 `status=ok` + `variations=[]`。
- 需優先使用 variation snapshots 補足 per-variation price。

## 8.5 Skill 5: `taobao-variation-image-map`
### 目的
離線建立規格到圖片映射。

### 驗收標準
- 每筆包含 `title`, `position`, `images[]`。
- item-level 失敗可跳過，不得整體報錯。
- 全部失敗可回 `status=ok` + `variations=[]`。
- 僅致命讀檔/解析錯誤才 `status=error`。

## 8.6 Skill 6: `taobao-orchestrator-pipeline`
### 目的
整合前述技能並輸出單一最終 contract。

### 驗收標準
- A(core) 成功時，即使 B/C/D 部分失敗，仍可 `status=ok`（degraded arrays）。
- pipeline state 檔案完整更新（run start/stage start/stage end/finalize）。
- `final.json` 與 stdout JSON 一致。

## 9. 正確性驗證方法
## 9.1 測試資料集
- 建立 `codex/.codex/taobao_test_urls.md`（或等效檔）管理測試 URL 與預期欄位。
- 每個 skill 至少跑 3 個 URL（正常/複雜/受限）。

## 9.2 驗證層次
1. Contract 驗證：JSON 欄位完整性與型別。
2. 內容驗證：
   - core：title 與頁面主標一致度
   - images：URL 可讀比例
   - variations：選項數與頁面可見規格一致度
3. 降級驗證：受阻頁必須穩定回 `needs_manual`。

## 10. 風險與緩解
- 風控升級導致 S0 不穩：
  - 緩解：將 blocked 判斷前置，明確 `needs_manual`，避免錯誤污染後續 stage。
- DOM 改版導致 parser 失效：
  - 緩解：每個 parser 至少 2 種提取路徑（selector + text/regex fallback）。
- SKU 多維組合太複雜：
  - 緩解：先支援主維度；多維映射列入下一輪增強。

## 11. 里程碑與時程（建議）
- M1（Day 1-2）：`taobao-page-snapshot` 通過驗收。
- M2（Day 2-3）：`taobao-product-core` 通過驗收。
- M3（Day 3-4）：`taobao-product-images` 通過驗收。
- M4（Day 4-5）：`taobao-product-variations` 通過驗收。
- M5（Day 5-6）：`taobao-variation-image-map` 通過驗收。
- M6（Day 6-7）：`taobao-orchestrator-pipeline` 整合驗收。

## 12. 進度追蹤 Checklist
### 12.1 Preflight
- [x] 建立測試 URL 清單（至少 3 類：正常/複雜/受限，參考 `codex/.codex/taobao_test_urls.md`）
- [x] 確認 artifact 路徑規範：`out/artifacts/<run_id>/`
- [x] 確認 JSON-only 輸出規範（skill stdout 不含其他文字；與 Shopee skill 規範一致）

### 12.2 Skill 1: `taobao-page-snapshot`
- [x] `SKILL.md` 初版完成
- [x] `scripts/cdp_snapshot_html.py` 可用或完成必要調整
- [x] 能輸出 `s0-initial.html.gz`
- [x] 能 best-effort 輸出 `s0-overlay.html.gz`
- [x] 能 best-effort 輸出 `s0-variation-<position>.html.gz`（最多 20）
- [x] 產出 `s0-manifest.json`
- [x] 產出 `s0-snapshot-pointer.json`
- [x] `tab_tracking` 欄位完整（含 close attempted/succeeded/error）
- [x] 受阻頁正確回 `status=needs_manual`
- [x] 驗收通過（可行性 + 正確性）

### 12.3 Skill 2: `taobao-product-core`
- [x] `SKILL.md` 初版完成
- [x] `extract_core_from_html.py` 初版完成
- [x] `status=ok` 時 `title/description/currency/price` 皆非空
- [x] `description` 長度上限 1500
- [x] 受阻頁正確回 `status=needs_manual`
- [x] 欄位不完整時正確回 `status=error`
- [x] 輸出 `core_extract.json`
- [x] 驗收通過（可行性 + 正確性）

### 12.4 Skill 3: `taobao-product-images`
- [x] `SKILL.md` 初版完成
- [x] `extract_images_from_html.py` 初版完成
- [x] 只保留 `http/https` URL
- [x] URL 去重
- [x] 上限 20 張
- [x] 無圖片時回 `status=ok` + `images=[]`
- [x] 致命錯誤才回 `status=error`
- [x] 輸出 `images_extract.json`
- [x] 驗收通過（可行性 + 正確性）

### 12.5 Skill 4: `taobao-product-variations`
- [x] `SKILL.md` 初版完成
- [x] `extract_variations_from_html.py` 初版完成
- [x] 每筆含 `title/position/price`
- [x] 去重後最多 20 筆
- [x] 可用 variation snapshots 補足 per-variation price
- [x] 無規格時回 `status=ok` + `variations=[]`
- [x] 致命錯誤才回 `status=error`
- [x] 輸出 `variations_extract.json`
- [x] 驗收通過（可行性 + 正確性）

### 12.6 Skill 5: `taobao-variation-image-map`
- [x] `SKILL.md` 初版完成
- [x] `extract_variation_image_map_from_html.py` 初版完成
- [x] 每筆含 `title/position/images[]`
- [x] item-level 失敗可跳過
- [x] 全部無結果時回 `status=ok` + `variations=[]`
- [x] 致命錯誤才回 `status=error`
- [x] 輸出 `variation_image_map_extract.json`
- [x] 驗收通過（可行性 + 正確性）

### 12.7 Skill 6: `taobao-orchestrator-pipeline`
- [x] `SKILL.md` 初版完成
- [x] stage 順序正確執行（S0 -> A -> B -> C -> D -> merge）
- [x] 每 stage 都會落地 artifact
- [x] `_pipeline-state.json` 具完整狀態流轉
- [x] `final.json` 與 stdout JSON 一致
- [x] A 成功但 B/C/D 部分失敗時可 degraded `status=ok`
- [x] 驗收通過（可行性 + 正確性）

### 12.8 Cross-Stage Regression
- [ ] 新增 skill 後，前一個 skill smoke test 仍通過
- [ ] 3 類 URL（正常/複雜/受限）回歸通過
- [ ] JSON contract 檢查通過（欄位/型別/必要值）

### 12.9 Release Readiness
- [ ] `codex/.codex/taobao_test_urls.md` 更新完成
- [ ] 已記錄已知限制與 fallback 規則
- [ ] 成功指標達標（依本文件 Success Metrics）

## 13. 成功指標（Success Metrics）
- 單一 skill 一次通過率（測試 URL 集）>= 90%。
- 受限頁正確降級率（needs_manual）>= 95%。
- `status=ok` 案例中，core 四欄完整率 >= 95%。
- pipeline 最終 JSON 合約違規率 = 0。

## 14. 開發執行原則
- 先可用，再擴充：每個 skill 先實作 MVP，再加強覆蓋率。
- 單一責任：每次 PR 僅處理一個 skill。
- 嚴格回歸：新 skill 合併前，至少重跑前一個已完成 skill 的 smoke test。

## 15. 建議第一個實作項目
優先從 `taobao-page-snapshot` 開始，原因：
1. 它是所有後續 offline parser 的資料來源。
2. 能最早暴露 Taobao 反爬與互動差異。
3. 一旦 snapshot 穩定，後面 skill 的迭代速度會明顯提升。
