/** 信用卡账单周期 / 还款日工具（与后端口径一致，账单日 1–28） */

function clampDay(year, monthIndex, day) {
  const d = Math.min(Math.max(day || 1, 1), 28)
  return new Date(year, monthIndex, d)
}

export function previousStatementDate(statementDay, from = new Date()) {
  const y = from.getFullYear()
  const m = from.getMonth()
  const day = from.getDate()
  const thisMonth = clampDay(y, m, statementDay)
  if (day >= statementDay) return thisMonth
  return clampDay(y, m - 1, statementDay)
}

/** offset=0 本账期，-1 上账期 */
export function billingCycleRange(statementDay, from = new Date(), offset = 0) {
  let last = previousStatementDate(statementDay, from)
  last = clampDay(last.getFullYear(), last.getMonth() + offset, statementDay)
  const start = new Date(last.getFullYear(), last.getMonth(), last.getDate() + 1, 0, 0, 0)
  const next = clampDay(last.getFullYear(), last.getMonth() + 1, statementDay)
  const end = new Date(next.getFullYear(), next.getMonth(), next.getDate(), 23, 59, 59)
  return { start, end }
}

export function formatDateYMD(d) {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
