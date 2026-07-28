# kumo PR/issue triage session — 完了

## 2026-07-27 追加分
- [x] PR #855 (myohei, BatchGetItem 応答形状) merge
- [x] issue #862 → PR #863 (SFN: execution ARN 小文字化 / States.Runtime 区別 / accessor の data race 修正) merge
- [x] issue #860 → PR #864 (DynamoDB: Limit を filter 前の評価数に適用、golden 再生成) merge
- [x] #858 → PR #866 merged (APIGWv2 JWT authorizer、issue 自動 close)
- [x] #859 → PR #865 merged (Lambda sync/async 直列化、issue 自動 close)
- [x] #861 → PR #867 merged (SFN Choice/Wait/Succeed/Fail + 素の Lambda ARN)。
  issue は open のまま (Parallel/Map/Retry/Catch が後続タスク)
- [ ] PR #854 (tagpr v0.26.1 release) — ユーザー判断待ち。merge すると tag + GitHub release

## Merged (12 PR)
- [x] #851 test(lambda) UUID path prefixes
- [x] #683 fix(messaging) SNS/SQS name validation
- [x] #673 fix(dynamodb) key attribute updates (rebase + lint fix)
- [x] #589 terraform/opentofu scaffold (service 側重複を drop、テストのみ)
- [x] #687 fix(kinesis) CreateStream validation (ValidationException 復元)
- [x] #856 fix: create-time tags (DynamoDB/EventBridge/SNS + formToJSON) — main green 復旧
- [x] #675 fix(s3) empty multipart → MalformedXML (main 構造へ移植)
- [x] #695 fix(s3) GET bucket notification (+Lambda configs)
- [x] #582 feat(amp) Managed Prometheus (squash-port)
- [x] #697 fix(s3) multipart versioning
- [x] #857 terraform e2e fixture-directory 化 (plan 冪等性 + 両 binary matrix)

## Issue close (10)
- [x] 自動: #670 #672 #682 #686 #694 #696 #708
- [x] 手動 (main で修正済み確認): #605 #611 #655

## ユーザー判断待ち (レビュー済み・レポート提出済み)
- #850: hard blocker なし、軽微 2 件 → merge 推奨
- #667/#668: メンテナ承認の形跡なし、機能方針の判断が必要
- #571: 指摘対応済みだが QuickJS/wazero 依存追加の判断が必要
- #816: NEEDS_WORK (entities 未対応 / Storage interface 不在 / path traversal 等 5 件)
- #509: NEEDS_WORK (要 rebase + Meta()/registry 対応)
- #419: 半分 stale。awsquery の MaxBytesReader だけ拾い直す価値あり
- draft #826-829: terraform refresh 系。#857 の fixture 方式で受け皿はできた
