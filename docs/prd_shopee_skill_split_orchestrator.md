# PRD: Shopee 拆分式 Skills + Orchestrator（解決 Gemini JSON 截斷）

## 1. 背景 / 問題

目前 `shopee-product-crawler` skill 會在單次執行中同時完成：
- 開頁與等待 ready
- 抽取核心欄位（title/description/price/currency/status）
- 抽取 images（overlay thumbnails）
- 抽取 variations（選項 + hover 對應圖片）

在 Gemini CLI（尤其是 tool-use + 長內容頁面）下，模型輸出可能出現：
- **JSON 被截斷（缺少結尾 `]` / `}`）**
- **缺少後半段欄位（常見是 `variations`）**
- 進而導致 parser / contract validation 失敗，後續 `productdrafts` 失敗（FAILED）。

根因不是「解析器問題」，而是「單次輸出太長、模型/CLI 早停」：長 description + 多張圖片 URL + variations mapping，容易超過安全輸出範圍。

## 2. 目標（Goals）

1) 讓 Gemini 輸出更穩定：每次輸出更短，**JSON 完整閉合** 的成功率提升。
2) 允許「降級成功」：至少能穩定拿到核心欄位（title/price/currency/description），images/variations 可選擇性補齊。
3) 可觀測性更清楚：分段抓取後，能快速定位是哪一段（core/images/variations）失敗。

## 3. 非目標（Non-Goals）

- 不追求單次執行最短時間；允許多階段抓取增加耗時。
- 不保證 images/variations 100% 完整；只保證「輸出格式穩定」與「核心欄位優先成功」。
- 不在本 PRD 中規劃跨站點（非 Shopee）泛化。

## 4. 方案概述

將目前單一大型 skill 拆成多個「輸出短、責任清楚」的 skills，並由一個 orchestrator 依序執行、合併結果。

### 4.0 推薦架構選擇：Snapshot（Master State）優先

本案採用 **「一次抓取 master state（snapshot），後續 stages 離線解析」** 作為預設路徑，以降低：
- 重複開頁/重複操作造成的 Shopee 風控（驗證牆）機率
- tool-use 流程不穩導致的中途失敗
- LLM 單次輸出過長導致的 JSON 截斷

原則：
- **只對真實網頁做一次操作**（snapshot skill / snapshot stage）
- 後續 core/images/variations/mapping stages 皆**只讀 snapshot artifacts**，輸出短 JSON
- 本 PRD 的預設範圍 **不包含**「各 stage 各自開頁」的路徑（避免引入重複操作與風控風險）

Snapshot 內容必須是「混合快照」而非只有 HTML：
- `outerHTML`（渲染後 DOM）
- `document.title`
- `meta`（og:title/og:description/price/currency 等）
- JSON-LD（`script[type="application/ld+json"]` 全量）
- 站內可能的內嵌 state blobs（例如全域變數/inline JSON；以實測 selector/key 為準）
- 若需要 images/variation-image-map 高成功率：snapshot 階段需先打開 overlay（modal）後，再抓 overlay 內縮圖 URLs / 相關 DOM/state

### 4.1 Skills 拆分建議

**Skill S0：shopee-page-snapshot（唯一會操作真實網頁的 skill）**
- 目的：使用 Chrome DevTools MCP 對目標 URL 做一次性快照，產出 master state artifacts。
- 輸出：`{ "snapshot_path": "string", "status": "ok|needs_manual|error", ... }`（短 JSON）
- 行為要點：
  - 偵測 blocked wall（login/verification/captcha）時回 `needs_manual`，並保存可診斷的最小資訊（例如 title/少量 HTML preview）
  - 對 images / variation image map 有需求時，snapshot 階段先打開 overlay（modal）再擷取 overlay 相關 DOM/state
  - Snapshot 產物要能在 7 天內回放/debug（見第 12 章）

**Skill A：shopee-product-core**
- 目的：從 snapshot artifacts 解析核心欄位（title/description/price/currency）與狀態（ok/needs_manual/error）。
- 輸出：短 JSON（只含 core 所需欄位；不含 images/variations）。
- 重要規則：
  - `description` 長度上限（例如 1500 chars）
  - 若 blocked（login/captcha），回 `needs_manual`

**Skill B：shopee-product-images**
- 目的：從 snapshot artifacts 解析圖片 URLs（overlay thumbnails）。
- 輸出：`{ "images": [...] }`，並限制最多 N 張（例如 20）。
- 重要規則：
  - 只回傳 URL list，不回傳 description/price（避免輸出膨脹）

**Skill C：shopee-product-variations**
- 目的：從 snapshot artifacts 解析 variations 選項文字（不做 hover 圖片對應）。
- 輸出：`{ "variations": [{title, position}] }`，限制最多 N（例如 20）。

**Skill D（可選）：shopee-variation-image-map**
- 目的：從 snapshot artifacts 取得 variation → image mapping（snapshot 階段已完成必要互動與資料擷取）。
- 輸出：`{ "variations": [{title, position, image}] }`
- 風險最高（耗時與風控），建議設為 feature flag，並加上硬上限避免拖垮整體成功率：
  - 最多只處理前 **10** 個 options（依 DOM 順序，0-based position）
  - 單一 option hover/click 後等待上限（例如 300ms）
  - 任一 option 失敗：**跳過該 option**、繼續下一個（整體仍可維持 `status="ok"`，但該 option 的 `image` 可能缺省）

### 4.2 Orchestrator 的責任

Orchestrator 會執行 **S0(snapshot) → A(core) → B(images) → C(variations) → D(mapping)**，最後組成最終物件。

核心責任：
- **Single-navigation 原則**：只有 S0 會操作真實網頁；A/B/C/D 一律離線解析 snapshot。
- **執行序控制**：依賴順序（核心先成功再補其他）。
- **合併策略**：
  - `core` 產生最終基礎物件（含 status/url/captured_at/title/description/currency/price）。
  - `images` 合併進 `images` 欄位；失敗時保留 `images: []` 或不覆蓋。
  - `variations` 同理。
- **輸出大小控制**：在 orchestrator 層面再次 enforce 上限（images/variations N、description 長度）。
- **失敗降級**：
  - A 失敗 → 直接回 error/needs_manual（不執行後續）
  - B/C/D 失敗 → 仍可回 ok（但 arrays 空或缺），或回 needs_manual（依策略）
- **可觀測性**：
  - log 每個階段的開始/結束/耗時/狀態
  - 附上 `stage_errors`（不一定要進 contract，可作為 runner 附加欄位或 debug log）
 - **Snapshot 管控**：
   - orchestrator 先產生/載入 snapshot（master state artifacts）
   - 若 snapshot 判定 blocked 或資料不足：直接回 `needs_manual` / `error`（不啟用「各 stage 各自開頁」的 fallback）

## 5. 資料契約（Contracts）

### 5.1 最終輸出契約（維持現有 CrawlOut）

最終仍輸出一個 JSON object，符合既有 contract（簡化表示）：
```json
{
  "url": "string",
  "status": "ok|needs_manual|error",
  "captured_at": "RFC3339",
  "notes": "string",
  "error": "string",
  "title": "string",
  "description": "string",
  "currency": "string",
  "price": "number|string",
  "images": ["string"],
  "variations": [{"title":"string","position":0,"image":"string"}]
}
```

### 5.2 分段技能輸出（建議）

- A（core）只需要輸出最終 contract 的必要欄位（或至少包含 status/url/captured_at）。
- B 只輸出 `{ "images": ["..."] }`
- C/D 只輸出 `{ "variations": [...] }`

Orchestrator 會負責把 B/C/D merge 回最終 contract。

## 6. 使用者故事（User Stories）

1) 當我輸入一個 Shopee 商品 URL，我希望大多數情況下都能拿到 **可解析的 JSON**，不要因為截斷而整筆失敗。
2) 就算 images/variations 有時候抓不到，我仍希望 core 欄位能成功，讓後續流程不中斷。
3) 當失敗發生時，我希望能快速知道卡在哪個 stage，方便修 prompt 或調整上限。

## 7. 成功指標（Metrics）

建議觀測：
- `contract_ok_rate`：最終輸出通過 contract validation 的比例
- `truncation_rate`：出現 “invalid or truncated JSON” 的比例（按 stage）
- `core_ok_rate`：核心欄位成功率（status=ok 且含 price/currency/title/description）
- 平均耗時 / P95 耗時（拆分後可能上升）

## 8. 風險與對策（Risks & Mitigations）

1) **耗時上升**：多次 DevTools 操作
   - 對策：只在 core ok 後才執行 images/variations；可用 feature flag 控制。

2) **Shopee 風控更容易觸發**
   - 對策：降低每階段工具呼叫數、節流、重試次數上限、必要時只跑 core。

3) **一致性（不同階段抓到不同快照）**
   - 對策：接受 eventual consistency；以 core 的 captured_at 為主；或 orchestrator 提供 `captured_at_core/images/...`（可選）。

4) **技能啟用失敗（skills 未部署到 server / activate_skill fail）**
   - 對策：部署時同步 skills；並在 worker 啟動時做一次 health check/預檢（非本 PRD 範圍）。

## 9. Rollout 計畫（高層級）

1) 先上線拆分後的 core + images（A+B），variations 先關閉。
2) 觀測 contract_ok_rate 與 truncation_rate 是否明顯改善。
3) 再逐步啟用 variations（C），最後才考慮 image mapping（D）。

## 10. 驗收條件（Acceptance Criteria）

必須達成：
- 在「圖片多/描述長」的頁面，最終輸出 JSON 截斷率明顯下降（以實際線上樣本衡量）。
- core 階段（A）成功時，最終輸出一定是可解析且符合 contract 的單一 JSON 物件。
- B/C/D 任一階段失敗不會導致整筆 crash；能以降級策略完成輸出。

---

## 11. Implementation TODOs（Progress Checklist）

> 目的：把工作拆成可驗收的 checkpoints，方便每一步做 progress check。

### 11.1 設計確認（Design Lock）
- [ ] 決定要先上線的最小組合：本案採 **`A(core) + B(images) + C(variations) + D(variation image map)` 一起做**（D 需硬上限 + 可降級）。
- [ ] 確認 single-navigation 原則：只有 `S0(snapshot)` 會操作真實網頁；A/B/C/D 皆離線解析 snapshot。
- [ ] 定義每個 stage 的輸出上限（建議值）：
  - [ ] `description_max_chars = 1500`
  - [ ] `images_max = 20`
  - [ ] `variations_max = 20`
- [ ] 定義 D 的執行上限（建議值）：
  - [ ] `variation_image_map_max = 10`（只處理前 10 個 options）
  - [ ] `variation_image_map_wait_ms = 300`（單 option hover/click 後等待上限）
  - [ ] 任一 option 失敗：跳過，不影響整體輸出
- [ ] 決定合併策略（merge precedence）：
  - [ ] core 成功才跑後續
  - [ ] images/variations 失敗時是否保留空陣列或不覆蓋
- [ ] 決定 orchestrator 的最終 status 規則：
  - [ ] core=`needs_manual` → 不跑後續，直接回 needs_manual
  - [ ] core=`error` → 不跑後續，直接回 error
  - [ ] core=`ok` + B/C/D 部分失敗 → 是否仍回 ok（降級）

### 11.2 Skills 拆分（Prompt/Skill）
- [ ] 新增 skill：`shopee-page-snapshot`
  - [ ] 只做一次性快照（outerHTML/meta/JSON-LD/state blobs）
  - [ ] 若需 images/mapping：在 snapshot 階段先打開 overlay 再擷取 overlay 相關 DOM/state
  - [ ] 產出 snapshot artifact（`snapshot_path`）供後續 stages 使用
- [ ] 新增 skill：`shopee-product-core`
  - [ ] 從 snapshot artifacts 解析 `title/description/price/currency`
  - [ ] 確保輸出短且一定閉合 JSON
- [ ] 新增 skill：`shopee-product-images`
  - [ ] 從 snapshot artifacts 解析 overlay thumbnails URLs，限制 `images_max`
- [ ] 新增 skill：`shopee-product-variations`（選配）
  - [ ] 從 snapshot artifacts 解析選項文字（不做 hover mapping），限制 `variations_max`
- [ ] 新增 skill：`shopee-variation-image-map`
  - [ ] 從 snapshot artifacts 取得/解析 mapping（互動已在 snapshot 階段完成）
  - [ ] 限制最多 options=10
  - [ ] 失敗跳過單一 option，不影響整體 stage 完成
- [ ] 每個 skill 都遵守：JSON ONLY、固定 key 集合、必填欄位策略一致

### 11.3 Orchestrator（組裝與降級）
- [ ] 新增 orchestrator 流程（stage pipeline）：S0(snapshot) → A → B → C → D（可配置開關）
- [ ] 實作結果合併（merge）：
  - [ ] 以 core 為 base
  - [ ] images/variations 合併並 enforce 上限
  - [ ] 保證最終輸出符合既有 contract（`variations`/`images` 缺省時填 `[]`）
- [ ] 增加 observability：
  - [ ] log `stage_started` / `stage_finished` / `stage_duration_ms`
  - [ ] log stage 失敗原因（但避免把超長 raw 全部打進 logs）

### 11.4 配置與開關（Runtime Flags）
- [ ] 新增 env/config 開關（建議）：
  - [ ] `SHOPEE_ORCH_ENABLED=1`
  - [ ] `SHOPEE_ORCH_IMAGES=1`
  - [ ] `SHOPEE_ORCH_VARIATIONS=0/1`
  - [ ] `SHOPEE_ORCH_VARIATION_IMAGE_MAP=0/1`（建議預設 1，但有硬上限）
  - [ ] `SHOPEE_ORCH_VARIATION_IMAGE_MAP_MAX=10`
- [ ] 確認預設值安全（不設 env 也能跑，且偏向穩定 core）

### 11.5 測試（Regression + Contract）
- [ ] 單元測試：merge 行為（core ok + images fail 仍可 ok）
- [ ] 單元測試：上限 enforcement（images/variations 截斷、description 截短）
- [ ] 單元測試：core needs_manual / error 時不跑後續 stages
- [ ] （可選）整合測試：用 mock runner 輸出模擬各 stage 成功/失敗組合

### 11.6 部署與驗證（VPS）
- [ ] 更新遠端 skills 部署（確保 `activate_skill` 不再 fail）
- [ ] 用 5–10 個「長描述 + 多圖」的 Shopee URL 做 smoke test
- [ ] 觀察 metrics / logs：
  - [ ] `invalid or truncated JSON` 明顯下降
  - [ ] `product_draft_upserted_from_crawl` 成功率提升

---

## 12. Artifacts 儲存策略（7 天保留）

結論：**以 filesystem（容器內 `/out`）作為每個 stage/skill artifact 的主要落地點**；SQLite 僅存「索引/摘要」（可選），避免 DB 膨脹。

### 12.1 為什麼選 filesystem 優先
- 本專案現況已經會把 crawl 結果寫到 `out/*.json`，且 `docker-compose.yml` 已將 `./out` bind-mount 到容器 `/out`，延用成本最低、debug 最直覺。
- stage artifacts 可能很大（長 description、多張 images URL、variations mapping 等），直接塞 SQLite 會快速膨脹，清理與觀測也更麻煩。

### 12.2 建議 artifacts 目錄結構

每次 orchestrator 處理一個 URL 產生一個 `run_id`（可用既有 `event_id`/urlsha256 或新 UUID），將中間結果落地：
- `/out/artifacts/<run_id>/snapshot.json`
- `/out/artifacts/<run_id>/core.json`
- `/out/artifacts/<run_id>/images.json`
- `/out/artifacts/<run_id>/variations.json`（選配）
- `/out/artifacts/<run_id>/variation_image_map.json`（選配）
- `/out/artifacts/<run_id>/final.json`
- `/out/artifacts/<run_id>/meta.json`（建議：每段耗時、錯誤、重試次數、model/skill 名稱）

### 12.3 SQLite（可選）只存索引/摘要

若需要查詢與報表，可在 SQLite 存一筆索引（不要存大 JSON）：
- `run_id`
- `url`
- `final_status`
- `created_at`
- `artifact_dir`（例如 `/out/artifacts/<run_id>`）
- `stage_statuses` / `durations_ms`（可用 JSON 欄位或扁平欄位）

### 12.4 7 天保留策略（Cleanup）

- 保留窗口：**7 天**
- 清理對象：`/out/artifacts/*` 內 `created_at`（或資料夾 mtime）超過 7 天的目錄整批刪除。
- 清理時機（擇一）：
  - worker 啟動時做一次清理
  - 每日排程（cron/runner）清理
  - 每 N 次 run（例如每 100 次）抽樣觸發一次清理

驗收：
- `/out` 使用量長期維持可控（不無限成長）
- 遇到問題時，7 天內仍可追溯每個 stage 的輸出與 merge 過程
