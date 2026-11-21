import type { MarketDepthSnapshot, SymbolSummary } from '../types/market'
import { apiClient } from './client'

const DEFAULT_SNAPSHOT_LEVELS = 20

export async function getAvailableSymbols(): Promise<string[]> {
  const response = await apiClient.get<string[]>('/symbols')
  return response.data
}

export async function getSymbolSummary(
  symbol: string,
): Promise<SymbolSummary> {
  const response = await apiClient.get<SymbolSummary>(
    `/engine/${encodeURIComponent(symbol)}/summary`,
  )

  return response.data
}

export async function getMarketDepth(
  symbol: string,
  levels = DEFAULT_SNAPSHOT_LEVELS,
): Promise<MarketDepthSnapshot> {
  const response = await apiClient.get<MarketDepthSnapshot>(
    `/engine/${encodeURIComponent(symbol)}/snapshot/${levels}`,
  )

  return response.data
}
