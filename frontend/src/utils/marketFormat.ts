export function formatMarketPrice(price: number | null) {
  if (price === null) {
    return '—'
  }

  return new Intl.NumberFormat('tr-TR', {
    style: 'currency',
    currency: 'TRY',
    minimumFractionDigits: 2,
  }).format(price / 100)
}

export function formatQuantity(quantity: number) {
  return quantity.toLocaleString('tr-TR')
}

export function formatTradeTime(createdAt: string) {
  const date = new Date(createdAt)

  if (Number.isNaN(date.getTime())) {
    return '—'
  }

  return new Intl.DateTimeFormat('tr-TR', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}
