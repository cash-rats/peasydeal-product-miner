# PRD: Shopee Skill-Native Orchestrator Pipeline

## 1. Summary

本 PRD 定義一個 **skill-native** 的 Shopee 抓取流程：

- Go runner **只呼叫一個 skill**：`shopee-orchestrator-pipeline`
- stage 編排由 orchestrator skill 內部負責：`snapshot_capture -> core_extract -> images_extract -> variations_extract -> variation_image_map_extract`
- 每個 stage 完成後都要落 artifact + 更新 pipeline state
- 最後由 orchestrator skill merge 成單一最終 JSON，回給 Go 做既有 contract validation 與後續 persistence

決策日期：**2026-02-09**

---

## 2. Context / Problem

現況單一 skill（`shopee-product-crawler`）在一次輸出中包含 core/images/variations/mapping，容易在 Gemini/Codex tool-use 長輸出場景發生 JSON 截斷，造成 parser 與 contract validation 失敗。

既有 PRD（`docs/prd_shopee_skill_split_orchestrator.md`）已定義拆分方向。本 PRD 進一步明確化為：

- 編排邏輯不放在 Go orchestration 程式
- 改由一個 orchestrator skill 執行完整 pipeline（含 artifacts/state 管理）

---

## 3. Goals

1. 將長輸出拆短，提升 JSON 完整閉合率與 contract 通過率。
2. 保留 single-navigation：只有 `snapshot_capture`（舊名 S0）操作真實頁面，後續皆離線解析 artifacts。
3. 提供可追溯的 stage artifacts 與 `_pipeline-state.json`，提升 debug 可觀測性。
4. 讓 Go runner 職責簡化為「呼叫 orchestrator skill + validate/persist 最終 JSON」。

---

## 4. Non-Goals

1. 不在本期處理跨平台（非 Shopee）泛化。
2. 不改既有最終輸出 contract（`status/url/captured_at/...`）。
3. 不保證 images/variations/mapping 100% 完整；核心以 core 成功率優先。

---

## 5. Scope

## In Scope

1. 新增/定義以下 skills：
   - `shopee-orchestrator-pipeline`
   - `shopee-page-snapshot`（stage: `snapshot_capture`，舊名 S0）
   - `shopee-product-core`（stage: `core_extract`，舊名 A）
   - `shopee-product-images`（stage: `images_extract`，舊名 B）
   - `shopee-product-variations`（stage: `variations_extract`，舊名 C）
   - `shopee-variation-image-map`（stage: `variation_image_map_extract`，舊名 D）
2. 定義 orchestrator skill 的 stage 執行、artifact 寫入、state 更新、merge 規則。
3. 定義 runtime flags（可由環境變數注入到 runtime prompt）。

## Out of Scope

1. 重寫 Go 端 DB schema 或 worker 主流程。
2. 改變下游 `productdrafts` 寫入模型。

---

## 6. Target Architecture

## 6.1 Execution Model

1. Go runner 對單一 URL 建立一次執行請求。
2. Go runner 呼叫 `shopee-orchestrator-pipeline`（skill mode）。
3. orchestrator skill 內部執行：
   - Stage `snapshot_capture`（舊名 S0）: 產生 snapshot artifacts
   - Stage `core_extract`/`images_extract`/`variations_extract`/`variation_image_map_extract`: 依序離線解析並產出 stage artifacts
   - Final merge: 組合最終 contract JSON
4. Go runner 僅做：
   - JSON 抽取/修復（既有能力）
   - contract validation（既有能力）
   - persist output（既有能力）

## 6.2 Single-Navigation Rule

硬規則：

1. 只有 `shopee-page-snapshot` 可以呼叫 chrome-devtools 進行真實頁面互動。
2. `core_extract`/`images_extract`/`variations_extract`/`variation_image_map_extract` 僅可讀 `out/artifacts/<run_id>/` 內檔案，不得再開頁。

## 6.3 Stage Name Mapping

為了避免縮寫模糊，文件與 state key 以 descriptive names 為主：

1. `snapshot_capture`（舊名 `S0`）
2. `core_extract`（舊名 `A`）
3. `images_extract`（舊名 `B`）
4. `variations_extract`（舊名 `C`）
5. `variation_image_map_extract`（舊名 `D`）

---

## 7. Artifact and State Contract

## 7.1 Directory Layout

每次 pipeline run 必須建立：

`out/artifacts/<run_id>/`

其中 `<run_id>` 建議格式：`YYYYMMDDThhmmssZ_<url_sha_or_rand>`

必要檔案：

1. `_pipeline-state.json`
2. `s0-snapshot-pointer.json`
3. `s0-page_state.json`
4. `s0-page.html`
5. `a-core.json`
6. `b-images.json`
7. `c-variations.json`
8. `d-variation-image-map.json`
9. `final.json`
10. `meta.json`

允許缺省（以空結果表示），但檔案仍建議存在以利 debug。

## 7.2 `_pipeline-state.json`

每次 stage transition 都必須更新：

```json
{
  "run_id": "20260209T120000Z_ab12cd",
  "url": "https://shopee.tw/...",
  "started_at": "2026-02-09T12:00:00Z",
  "updated_at": "2026-02-09T12:00:30Z",
  "current_stage": "images_extract",
  "status": "running|completed|needs_manual|error",
  "stages": {
    "snapshot_capture": {"status": "completed", "started_at": "...", "ended_at": "...", "error": ""},
    "core_extract": {"status": "completed", "started_at": "...", "ended_at": "...", "error": ""},
    "images_extract": {"status": "running", "started_at": "...", "ended_at": "", "error": ""},
    "variations_extract": {"status": "pending", "started_at": "", "ended_at": "", "error": ""},
    "variation_image_map_extract": {"status": "pending", "started_at": "", "ended_at": "", "error": ""}
  },
  "flags": {
    "images_enabled": true,
    "variations_enabled": true,
    "variation_image_map_enabled": true
  }
}
```

## 7.3 `meta.json`

儲存可觀測性資訊：

```json
{
  "run_id": "string",
  "tool": "codex|gemini",
  "orchestrator_skill": "shopee-orchestrator-pipeline",
  "stage_duration_ms": {"snapshot_capture": 1200, "core_extract": 80, "images_extract": 60, "variations_extract": 40, "variation_image_map_extract": 110},
  "stage_errors": [],
  "limits": {
    "description_max_chars": 1500,
    "images_max": 20,
    "variations_max": 20,
    "variation_image_map_max": 10
  }
}
```

---

## 8. Stage Specifications

## 8.1 Stage `snapshot_capture`（舊名 S0）: `shopee-page-snapshot`

輸入：

1. 目標 URL
2. 限制設定（images/variations/mapping 上限）

輸出：

1. `s0-snapshot-pointer.json`（小 JSON）
2. `s0-page_state.json`
3. `s0-page.html`
4. overlay / variations / mapping 原始 artifacts（可為空）

規則：

1. 遇 login/captcha/verification 且核心不可見時回 `needs_manual`。
2. 必須輸出可 parse JSON，不可輸出 markdown/prose。

## 8.2 Stage `core_extract`（舊名 A）: `shopee-product-core`

輸入：`snapshot_capture` artifacts  
輸出：`a-core.json`

最低欄位：

```json
{
  "status": "ok|needs_manual|error",
  "title": "string",
  "description": "string",
  "currency": "string",
  "price": "number|string",
  "notes": "string",
  "error": "string"
}
```

規則：

1. `description` 最長 1500 chars。
2. `core_extract` 為 gate：其狀態非 `ok` 則 pipeline 直接結束（不跑 `images_extract`/`variations_extract`/`variation_image_map_extract`）。

## 8.3 Stage `images_extract`（舊名 B）: `shopee-product-images`

輸入：`snapshot_capture` artifacts  
輸出：`b-images.json`

```json
{"images": ["https://..."]}
```

規則：最多 20 張，去重。

## 8.4 Stage `variations_extract`（舊名 C）: `shopee-product-variations`

輸入：`snapshot_capture` artifacts  
輸出：`c-variations.json`

```json
{
  "variations": [{"title": "Red", "position": 0}]
}
```

規則：最多 20 筆。

## 8.5 Stage `variation_image_map_extract`（舊名 D）: `shopee-variation-image-map`

輸入：`snapshot_capture` artifacts  
輸出：`d-variation-image-map.json`

```json
{
  "variations": [{"title": "Red", "position": 0, "image": "https://..."}]
}
```

規則：

1. 最多處理前 10 options。
2. 單 option 失敗需跳過，不可讓整 stage hard fail。

---

## 9. Merge and Degradation Rules

orchestrator skill merge 順序：

1. 以 `core_extract` 結果為 base 建立最終物件。
2. `images_extract` 成功時覆蓋/填入 `images`，失敗則 `images=[]`。
3. `variations_extract` 成功時填入 `variations`（若 `variation_image_map_extract` 關閉）。
4. `variation_image_map_extract` 成功時用 `{title,position}` 對齊補 `variation.image`；找不到 mapping 時保留無 image。

狀態規則：

1. `core_extract`=`needs_manual` -> final=`needs_manual`（停止）。
2. `core_extract`=`error` -> final=`error`（停止）。
3. `core_extract`=`ok` 且 `images_extract`/`variations_extract`/`variation_image_map_extract` 任意失敗 -> final 仍可 `ok`（降級）。

最終輸出必須符合既有 contract，至少包含：

1. `url`
2. `status`
3. `captured_at`
4. `title/description/currency/price`（status=ok）
5. `images`（至少 `[]`）
6. `variations`（至少 `[]`）

---

## 10. Runtime Flags

建議由環境變數注入 orchestrator runtime prompt：

1. `SHOPEE_ORCH_ENABLED=1`
2. `SHOPEE_ORCH_IMAGES=1`
3. `SHOPEE_ORCH_VARIATIONS=1`
4. `SHOPEE_ORCH_VARIATION_IMAGE_MAP=1`
5. `SHOPEE_ORCH_DESCRIPTION_MAX_CHARS=1500`
6. `SHOPEE_ORCH_IMAGES_MAX=20`
7. `SHOPEE_ORCH_VARIATIONS_MAX=20`
8. `SHOPEE_ORCH_VARIATION_IMAGE_MAP_MAX=10`

預設策略：

1. orchestrator 啟用時 `core_extract`/`images_extract`/`variations_extract`/`variation_image_map_extract` 全開。
2. 允許緊急降載時關閉 `variations_extract` 或 `variation_image_map_extract`。

---

## 11. Rollout Plan

## Phase 1: Skill Contracts

1. 補齊 5+1 skills 契約與輸出格式。
2. 先在本機 `make dev-once` 驗證 artifacts 與 final JSON。

## Phase 2: Runner Wiring

1. Go 預設 skill 改為 `shopee-orchestrator-pipeline`（可用 flag 切回舊 skill）。
2. 維持既有 contract validator 不變。

## Phase 3: Soak

1. 使用 5-10 個長描述/多圖頁面做 smoke test。
2. 比較 `contract_ok_rate`、`truncation_rate`、`core_ok_rate`、P95 耗時。

---

## 12. Acceptance Criteria

1. 最終輸出 JSON 截斷率相較現況顯著下降。
2. `core_extract` 成功時，最終輸出始終可解析且通過 contract validation。
3. `images_extract`/`variations_extract`/`variation_image_map_extract` 任一失敗不會 crash 整筆流程。
4. 每次 run 都能在 `out/artifacts/<run_id>/` 找到 stage artifacts 與 `_pipeline-state.json`。

---

## 13. Risks and Mitigations

1. Skill 內「讀另一個 skill」在不同工具支援度不一致。  
   對策：orchestrator SKILL.md 直接內嵌 stage 規格，必要時把 `core_extract`/`images_extract`/`variations_extract`/`variation_image_map_extract` 規格同步為 references，避免依賴工具的 skill-to-skill 呼叫能力。

2. artifacts 體積膨脹。  
   對策：保留 7 天清理，長文本截斷並標記 `*_truncated`。

3. 價格區間（price range）與現行 validator 不一致。  
   對策：先維持現行 numeric 規則；若遇 range，降級 `status=error` 並記錄原因，後續另開 PRD 調整 contract。

---

## 14. Open Questions

1. `price` 是否要升級為 `price_min/price_max` 模型？
2. orchestrator 是否需在 final JSON 增加 `stage_errors`（目前可先放 `meta.json`）？
3. 是否讓 orchestrator skill 成為 Shopee 預設 skill（取代 `shopee-product-crawler`）？

---

## 15. Implementation Checklist

- [x] 建立 `shopee-orchestrator-pipeline` skill（含 artifact/state 寫入規範）
- [x] 建立 `core_extract`/`images_extract`/`variations_extract`/`variation_image_map_extract` skills（固定短 JSON 輸出）
- [x] 對齊 `snapshot_capture` 輸出檔名與欄位到本 PRD 契約
- [x] 加入 `_pipeline-state.json` 更新規則
- [x] 實作 final merge 與降級策略於 orchestrator skill
- [x] 由 program 提供 `run_id`（worker 使用 `event_id`；dev-once 自動產生 UUID），並注入 skill prompt
- [ ] 補 smoke test 清單（5-10 URLs）
- [x] 更新 README 的 skill mode 指引與部署同步步驟
