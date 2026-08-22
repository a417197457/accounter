// 流水中的支付路径统一展示：支付渠道余额与关联银行卡必须可区分。
export function paymentRouteLabel(tx, fallback = '未知账户') {
  const account = tx?.account
  const paymentAccount = tx?.payment_account

  if (paymentAccount) {
    return [paymentAccount.name, account?.name].filter(Boolean).join(' · ')
  }
  if (account && (account.type === 'alipay' || account.type === 'wechat')) {
    return `${account.name}余额`
  }
  return account?.name || fallback
}
