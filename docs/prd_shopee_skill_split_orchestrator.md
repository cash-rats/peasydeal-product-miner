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

### 4.1 Skills 拆分建議

**Skill A：shopee-product-core**
- 目的：只抓核心欄位（title/description/price/currency）與狀態（ok/needs_manual/error）。
- 輸出：短 JSON（不含 images/variations 或只輸出空陣列）。
- 重要規則：
  - `description` 長度上限（例如 1500 chars）
  - 若 blocked（login/captcha），回 `needs_manual`

**Skill B：shopee-product-images**
- 目的：只抓圖片 URLs（overlay thumbnails）。
- 輸出：`{ "images": [...] }`，並限制最多 N 張（例如 20）。
- 重要規則：
  - 只回傳 URL list，不回傳 description/price（避免輸出膨脹）

**Skill C：shopee-product-variations**
- 目的：只抓 variations 選項文字（不做 hover 圖片對應）。
- 輸出：`{ "variations": [{title, position}] }`，限制最多 N（例如 20）。

**Skill D（可選）：shopee-variation-image-map**
- 目的：hover/click 變體選項，讀取 main image URL 對應。
- 輸出：`{ "variations": [{title, position, image}] }`
- 風險最高（耗時與風控），預設可先不啟用或設為 feature flag。

### 4.2 Orchestrator 的責任

Orchestrator 會執行 A →（成功時）B →（選配）C →（選配）D，最後組成最終物件。

核心責任：
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

