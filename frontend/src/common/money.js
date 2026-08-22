/** 账户余额展示：信用卡 balance≥0 为欠款，<0 为溢缴（多还的钱） */
export function formatAccountMoney(account, balance) {
  const raw = balance !== undefined ? balance : account?.balance
  const n = parseFloat(raw)
  const amount = Number.isFinite(n) ? Math.abs(n).toFixed(2) : '0.00'
  if (account?.type === 'credit') {
    if (n < 0) return { label: '溢缴', amount, isDebt: false, isCredit: true }
    return { label: '欠款', amount, isDebt: n > 0, isCredit: true }
  }
  return { label: '余额', amount: Number.isFinite(n) ? n.toFixed(2) : '0.00', isDebt: false, isCredit: false }
}

export function accountMoneyText(account, balance) {
  const info = formatAccountMoney(account, balance)
  return `${info.label} ${info.amount}`
}
