# PRD: Shopee Variation Image Map 支援多圖（`image` -> `images[]`）

## 1. Summary

本 PRD 定義 Shopee pipeline 的 variation image 格式升級：

- 單圖欄位：`variation.image`（string）
- 升級為多圖欄位：`variation.images`（string array）

目標是讓同一個 variation 可攜帶多張圖，並保持對既有資料的相容讀取能力。

決策日期：2026-02-10

---

## 2. Problem Statement

目前流程只允許每個 variation 對應一張圖（`image`），但實際商家商品可能在同一 variation 下有多張可用圖。現行格式造成：

1. 資訊遺失：只能保留第一張或任意一張。
2. 後續商品建檔彈性不足：無法在下游做更完整圖組策略。
3. stage artifacts 與最終輸出語意不一致（資料本質是 1:N，但 schema 是 1:1）。

---

## 3. Goals

1. 將 variation image schema 升級為多圖：`images: string[]`。
2. 端到端一致：S0 artifact、D stage artifact、final contract 同步升級。
3. 保留 backward compatibility：可讀舊格式 `image`。
4. 不破壞 pipeline 降級策略與 status 邏輯（`ok/needs_manual/error`）。

---

## 4. Non-Goals

1. 不改動 core 欄位（title/description/currency/price）判定規則。
2. 不改動整體 stage 順序與 orchestration 邏輯。
3. 不在本期改 DB schema（目前 payload 為 JSON passthrough）。

---

## 5. Scope

### In Scope

1. `shopee-page-snapshot`：`s0-variation_image_map.json` 產出格式升級。
2. `shopee-variation-image-map`：normalize 與輸出支援 `images[]`。
3. `shopee-orchestrator-pipeline`：D stage 與 final merge 規則改為多圖。
4. runner contract/prompt 對齊：允許 `variations[].images`。

### Out of Scope

1. 非 Shopee source 的 variations schema 調整。
2. productdrafts 下游業務欄位重構。

---

## 6. Data Contract Changes

## 6.1 S0 Artifact (`s0-variation_image_map.json`)

### New (target)

```json
{
  "status": "ok|error",
  "map": [
    {
      "title": "string",
      "position": 0,
      "images": ["https://..."]
    }
  ],
  "error": "string"
}
```

Rules:

1. `images` 必須存在，可為 `[]`。
2. URL 僅接受 `http/https`。
3. 每 item `images` 需去重並維持穩定順序。
4. item-level failure 可跳過，不應直接讓 stage hard-fail。

## 6.2 D Stage Artifact (`d-variation-image-map.json`)

### New (target)

```json
{
  "status": "ok|error",
  "variations": [
    {
      "title": "string",
      "position": 0,
      "images": ["https://..."]
    }
  ],
  "error": "string"
}
```

## 6.3 Final Output Contract

`variations` item 由：

```json
{"title":"string","position":0,"image":"string"}
```

升級為：

```json
{"title":"string","position":0,"images":["string"]}
```

---

## 7. Backward Compatibility Strategy

採兩階段遷移：

### Phase 1（相容期，建議先上）

1. 讀取端同時接受：
   - 舊：`image: string`
   - 新：`images: string[]`
2. 寫出端以新格式為主：`images[]`。
3. 可選：暫時雙寫 `image = images[0]` 供舊 consumer 使用。

### Phase 2（收斂期）

1. 移除寫出 `image`。
2. 僅保留 `images[]`。
3. 清理舊格式相容分支（需確認所有 consumer 已升級）。

---

## 8. Component Impact

1. Skills:
   - `codex/.codex/skills/shopee-page-snapshot/SKILL.md`
   - `codex/.codex/skills/shopee-variation-image-map/SKILL.md`
   - `codex/.codex/skills/shopee-orchestrator-pipeline/SKILL.md`
   - （若使用 Gemini 同步技能庫）對應 `gemini/.gemini/skills/...`

2. Runner contract & prompts:
   - `internal/runner/contract.go`
   - `internal/runner/gemini.go`
   - `internal/runner/codex.go`
   - `config/prompt.shopee.product.txt`（若仍會被 legacy mode 使用）

3. 測試:
   - `internal/runner/*_test.go` 補新舊格式共存測試。

---

## 9. Merge Rules Update

final merge（variation_image_map_extract -> final）調整為：

1. 以 `{title, position}` 對齊 variation。
2. 若 mapping 提供 `images[]`，合併到 variation.images。
3. 若僅有舊格式 `image`，轉為 `images=[image]`。
4. 同一 variation 多來源圖片去重，保序。
5. 非 core stage 失敗仍允許降級 `status=ok`（遵循既有規則）。

---

## 10. Validation Rules

1. `variations` 必須存在，可為空陣列。
2. `variations[].images` 必須存在，可為空陣列。
3. `variations[].images` 每項需為非空 URL 字串。
4. 保留既有 `status` 對應規則：
   - `ok`：需有完整 core fields
   - `needs_manual`：需有 notes
   - `error`：需有 error

---

## 11. Rollout Plan

1. 更新三個 Shopee skills schema（S0/D/final）。
2. 更新 runner contract + repair prompts。
3. 新增/調整測試（新舊格式都要過）。
4. 先以相容期（Phase 1）部署。
5. 觀察 1-2 週後，若無舊 consumer 依賴，再進入 Phase 2。

---

## 12. Test Plan

1. Unit test：
   - 舊格式輸入（`image`）可被正確轉成 `images[]`。
   - 新格式輸入（`images[]`）可通過 contract。
   - 混合格式（同時有 `image` + `images`）合併去重正確。
2. Integration test：
   - 跑一次 orchestrator pipeline，確認 artifacts 與 final.json 皆為新格式。
3. Regression test：
   - `status=needs_manual/error` 路徑不受本次 schema 變更影響。

---

## 13. Risks and Mitigations

1. 風險：LLM 仍輸出舊欄位 `image`。
   - 對策：在 skill 規範與 repair prompt 明確要求 `images[]`，並保留相容解析。

2. 風險：下游 consumer 還依賴 `image`。
   - 對策：Phase 1 雙寫與公告 cutover 時間。

3. 風險：多圖導致 payload 膨脹。
   - 對策：每 variation 設上限（例如 5），全體仍受既有限制控制。

---

## 14. Success Criteria

1. 新跑批產出中，`variations[].images` 覆蓋率達 100%。
2. contract validation pass rate 不低於改版前。
3. 無因 schema 變更導致的 productdrafts 寫入失敗。
4. 若開啟雙寫，舊 consumer 無中斷；下線 `image` 後無回歸。
