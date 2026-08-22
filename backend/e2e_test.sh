#!/bin/bash
set -euo pipefail
BASE=http://127.0.0.1:18080/api

json_get() {
  python3 -c "import json,sys; d=json.loads(sys.argv[1]); print(eval(sys.argv[2]))" "$1" "$2"
}

echo "== register A =="
REG=$(curl -s -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
  -d '{"username":"user_a","password":"pass1234","nickname":"甲"}')
echo "$REG"
TOKEN_A=$(json_get "$REG" "d['data']['token']")

echo "== register B =="
REG2=$(curl -s -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
  -d '{"username":"user_b","password":"pass1234","nickname":"乙"}')
TOKEN_B=$(json_get "$REG2" "d['data']['token']")

echo "== ledgers A =="
LEDGERS=$(curl -s "$BASE/ledgers" -H "Authorization: Bearer $TOKEN_A")
echo "$LEDGERS"
PERSONAL=$(python3 -c "import json,sys; ls=json.loads(sys.argv[1])['data']; print(next(x['id'] for x in ls if x['is_personal']))" "$LEDGERS")

ACCOUNTS=$(curl -s "$BASE/accounts" -H "Authorization: Bearer $TOKEN_A")
ACC_ID=$(json_get "$ACCOUNTS" "d['data'][0]['id']")
CATS=$(curl -s "$BASE/categories/flat?kind=expense" -H "Authorization: Bearer $TOKEN_A")
CAT_ID=$(json_get "$CATS" "d['data'][0]['id']")
DAY=$(date +%Y-%m-%d)

echo "== create tx =="
TX=$(curl -s -X POST "$BASE/transactions" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d "{\"account_id\":$ACC_ID,\"category_id\":$CAT_ID,\"op_type\":\"expense\",\"amount\":\"30.50\",\"remark\":\"打车\",\"occurred_at\":\"$DAY 12:00:00\"}")
echo "$TX"
TX_ID=$(json_get "$TX" "d['data']['id']")

echo "== create travel ledger =="
FAM=$(curl -s -X POST "$BASE/ledgers" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d '{"name":"测试旅行","type":"travel"}')
echo "$FAM"
echo "$FAM" | grep -q '"type":"travel"' || { echo "FAIL: travel ledger type missing"; exit 1; }
FAM_ID=$(json_get "$FAM" "d['data']['ledger']['id']")
INVITE=$(json_get "$FAM" "d['data']['invite_code']")

echo "== join travel ledger =="
JOIN=$(curl -s -X POST "$BASE/ledgers/join" -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$INVITE\"}")
echo "$JOIN"

echo "== direct create on travel ledger (should fail) =="
# need target ledger account/category ids
FAM_ACC=$(curl -s "$BASE/accounts" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $FAM_ID")
FAM_ACC_ID=$(json_get "$FAM_ACC" "d['data'][0]['id']")
FAM_CAT=$(curl -s "$BASE/categories/flat?kind=expense" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $FAM_ID")
FAM_CAT_ID=$(json_get "$FAM_CAT" "d['data'][0]['id']")
DIRECT=$(curl -s -X POST "$BASE/transactions" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $FAM_ID" -H 'Content-Type: application/json' \
  -d "{\"account_id\":$FAM_ACC_ID,\"category_id\":$FAM_CAT_ID,\"op_type\":\"expense\",\"amount\":\"10\",\"remark\":\"直记\",\"occurred_at\":\"$DAY 12:00:00\"}")
echo "$DIRECT"
echo "$DIRECT" | grep -q '该账本不能直接记账' || { echo "FAIL: target ledger direct create not blocked"; exit 1; }

echo "== push =="
PUSH=$(curl -s -X POST "$BASE/transactions/$TX_ID/push" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $PERSONAL" -H 'Content-Type: application/json' \
  -d "{\"target_ledger_id\":$FAM_ID}")
echo "$PUSH"
echo "$PUSH" | grep -q '"source_id"' || { echo "FAIL: push missing source_id"; exit 1; }

echo "== push duplicate =="
PUSH2=$(curl -s -X POST "$BASE/transactions/$TX_ID/push" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $PERSONAL" -H 'Content-Type: application/json' \
  -d "{\"target_ledger_id\":$FAM_ID}")
echo "$PUSH2"
echo "$PUSH2" | grep -q '已同步' || { echo "FAIL: duplicate push not rejected"; exit 1; }

echo "== family txs =="
FAM_TX=$(curl -s "$BASE/transactions?page=1&page_size=20" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $FAM_ID")
echo "$FAM_TX"
TOTAL=$(json_get "$FAM_TX" "d['data']['total']")
[ "$TOTAL" = "1" ] || { echo "FAIL: expected 1 family tx, got $TOTAL"; exit 1; }

echo "== delete synced copy only =="
COPY_ID=$(json_get "$FAM_TX" "d['data']['list'][0]['id']")
COPY_DEL=$(curl -s -X DELETE "$BASE/transactions/$COPY_ID" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $FAM_ID")
echo "$COPY_DEL"
SOURCE_AFTER_COPY_DELETE=$(curl -s "$BASE/transactions/$TX_ID" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $PERSONAL")
echo "$SOURCE_AFTER_COPY_DELETE" | grep -q '"id"' || { echo "FAIL: deleting copy removed source"; exit 1; }

echo "== re-push after copy deletion =="
PUSH3=$(curl -s -X POST "$BASE/transactions/$TX_ID/push" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $PERSONAL" -H 'Content-Type: application/json' \
  -d "{\"target_ledger_id\":$FAM_ID}")
echo "$PUSH3"
echo "$PUSH3" | grep -q '"source_id"' || { echo "FAIL: re-push after copy deletion failed"; exit 1; }

echo "== delete source =="
DEL=$(curl -s -X DELETE "$BASE/transactions/$TX_ID" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $PERSONAL")
echo "$DEL"
FAM_TX2=$(curl -s "$BASE/transactions?page=1&page_size=20" -H "Authorization: Bearer $TOKEN_A" -H "X-Ledger-Id: $FAM_ID")
echo "$FAM_TX2"
TOTAL2=$(json_get "$FAM_TX2" "d['data']['total']")
[ "$TOTAL2" = "0" ] || { echo "FAIL: cascade delete failed, total=$TOTAL2"; exit 1; }

echo "== overview with range =="
# recreate one tx for overview
TX2=$(curl -s -X POST "$BASE/transactions" -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' \
  -d "{\"account_id\":$ACC_ID,\"category_id\":$CAT_ID,\"op_type\":\"expense\",\"amount\":\"12\",\"remark\":\"咖啡\",\"occurred_at\":\"$DAY 12:00:00\"}")
OV=$(curl -s "$BASE/stats/overview?start=$DAY&end=$DAY" -H "Authorization: Bearer $TOKEN_A")
echo "$OV"
echo "$OV" | grep -q '12' || { echo "FAIL: overview missing amount"; exit 1; }

echo "== ALL CHECKS PASSED =="
