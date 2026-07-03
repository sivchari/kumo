# 調査: lint 制約優先で冗長/可読性低下している箇所と簡素化案

対象: kumo リポジトリ全体。実 lint (`make lint`, pinned golangci-lint) は 0 issues。
効いている複雑度系 linter: **cyclop (max-complexity 15)、funlen (~60 行/40 文), gocognit (30), nestif (depth 5)**。
これらを満たすために `//nolint` や不自然な構造になっている箇所を洗い出し、**linter が自然に通る (nolint を削除できる) 形**へ簡素化する案。

実装方針: すべて **behavior-preserving**。各変更後に `golangci-lint run` で nolint 削除可否を確認し、golden が変わらないことを確認する。**PR #810 マージ後に新ブランチで着手**。

---

## Tier 1 — 局所的・低リスク・nolint 削除可（複雑度系。ユーザー主眼）

これらは「制約優先で冗長化した」典型。1 ファイル内で完結し、挙動不変、nolint を消せる。

### dynamodb
1. **`copyAttributeValue` (storage.go:1026) + `convertAttrToStreamAttr` (storage.go:2063)** — 最優先。
   13 フィールドを `if av.X != nil { x := *av.X; result.X = &x }` で逐次コピーした ~75 行の双子関数。両方 funlen nolint。
   案: `clonePtr[T any](*T) *T` / `cloneSlice[T]([]T) []T` / `cloneBytes` の generic ヘルパで各ブロックを 1 行化。両関数が閾値以下になり nolint 削除、かつ「2 つを手で同期する」危険も解消。risk: low
2. **`Query` (storage.go:642)** — cyclop+funlen+gocognit。~135 行。partition-key チェックが 3-4 段ネスト (689-697)。
   案: `itemMatchesQuery(...)`(ネストを early-return 化), `keyAttrName(keySchema, keyType)`(HASH/RANGE 走査の重複除去), `paginate(...)` を抽出。Query が resolve→filter→sort→paginate に。risk: low
3. **`UpdateItem` (storage.go:530)** — funlen+cyclop。末尾 ReturnValues switch に冗長な内側 if あり。
   案: `returnUpdateResult(returnValues, old, new)` 抽出 + UpdatedOld/UpdatedNew を独立 case 化して内側 if 削除。risk: low
4. **`compareAttributeValues` (condition.go:529)** — cyclop。BOOL/NULL の `switch op` が二重インライン。
   案: `equalityOnly(equal bool, op string) bool` 抽出 → BOOL/NULL が 1 行に。risk: low
5. **`extractPartitionKeyValue` (storage.go:1116)** — nestif。4 段ネストの guard。
   案: early `continue` で平坦化。risk: low
6. **`tableToDescription` (handlers.go:458)** — funlen。GSI/LSI 変換ループがインライン。
   案: `gsiToDescription` / `lsiToDescription` 抽出。risk: low
7. **`CreateTable` (storage.go:230)** — funlen 境界。Stream 設定ブロックがはみ出し。
   案: `setupStream(table, req)` 抽出。risk: low

### 他サービス
8. **s3 `handleBucketGet` (handlers.go:120)** — funlen。10 個の `if query.Has("X") { s.HandlerX(); return }`。
   案: 順序付き `[]struct{key string; handler}` テーブル + ループ。順序厳守。risk: low
9. **sqs `DispatchAction` (handlers.go:756)** — cyclop。17-case の X-Amz-Target switch。
   案: `map[string]func(*Service, w, r)` テーブル化（CLAUDE.md の JSON protocol パターンに沿う）。risk: low
10. **route53 `ChangeResourceRecordSets` (handlers.go:367)** + **`ListHostedZonesByName` (230)** — funlen。
    案: sentinel→HTTP マッピングをテーブル駆動の `writeChangeRecordSetError`、pagination を `parseMaxItems`/`paginate` 共有化。risk: low-med
11. **sns `matchesOperator` (storage.go:536)** — nestif。operator ごとに ok+Unmarshal の2段ネスト。
    案: `matchExists`/`matchPrefix`/`matchAnythingBut` に分割。risk: low
12. **sfn `resolveParameters` (executor.go:156)** — nestif。
    案: `resolveJSONPathRef` / `resolveStaticValue` 抽出（input の lazy single-parse は維持）。risk: low-med
13. **lambda `AddPermission` (869) / pipes `CreatePipe` (166) / emrserverless `CreateApplication` (173) / forecast `CreatePredictor` (482)** — funlen。必須フィールド guard の連打。
    案: 各パッケージに小さな `requireNonEmpty`/`requireFields` ヘルパ。forecast は lock 保持に注意。risk: low(-med)

---

## Tier 2 — 横断ボイラープレート（高インパクトだが要慎重・別軸）

ユーザーの「cyclop 等の制約」とは少しズレるが、ハンドラを冗長にしている根因。**範囲が広く golden 影響あり**。1 ヘルパずつ段階的に、golden 再生成しながら。

1. **`readJSONRequest` が 26 パッケージで完全同一コピー** → `service.ReadJSONRequest` に集約。挙動完全一致。risk: low
2. **`writeJSONResponse` が 21 パッケージでほぼ同一**（json-1.0/1.1 と requestid ヘッダ大小のみ差）→ `service.WriteJSONResponse(w, contentType, v)`。risk: low（ヘッダ大小は HTTP 上無影響だが golden 文字列は確認）
3. **`ServiceError{Code,Message}+Error()` が 18 パッケージで同一定義**（named 変種 TableError 等含め 30）→ `service.CodedError` 共有。risk: med（型変更の波及）
4. **decode 失敗ブロック (~379 箇所) / 必須フィールド guard (~250) / `errors.As+InternalServerError` fallback (~176)** → 各パッケージに `decodeJSON` / `requireField` / `writeStorageError` の小ヘルパ。高密度パッケージ (cloudwatch, sqs, ec2, dynamodb) から。risk: low だが量が多い
5. **エラーコード文字列リテラル**（"ValidationException" 264x 等）の定数化、**`writeError` のシグネチャ不統一（13 種、引数順が逆のものあり）** の正規化。readability/footgun 対策。risk: low-med

---

## 触らない（inherent — nolint は妥当）

- gosec (72) / gocritic hugeParam (AttributeValue 値渡し) / nilnil 等は複雑度・可読性問題ではない。
- s3 `CopyObject`/`ListObjects`, sfn `executeLambdaInvoke`, dynamodb `parseUpdateClauses` 等は「重複のない直線的な手順」で長さが本質的。
- test ファイルの funlen/cyclop nolint（連続シナリオの golden test）は意図的。

---

## 進め方の提案

- **Tier 1 を 1 PR 1〜数件**で（パッケージ単位 or 関数単位）。各 PR で nolint 削除 + `make lint` 0 + golden 不変を確認。dynamodb の #1 (clone ヘルパ) から着手が費用対効果◎。
- **Tier 2 は Tier 1 の後**。まず無害な #1/#2（readJSONRequest/writeJSONResponse 集約）→ 様子を見て #4 のヘルパ展開。型変更 (#3) と writeError 正規化 (#5) は最後。
- すべて #810 マージ後の新ブランチで。
