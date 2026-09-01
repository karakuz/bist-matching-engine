import type { SymbolSummary } from '../types/market'
import { apiClient } from './client'


export async function getSymbolSummary(
  symbol: string,
): Promise<SymbolSummary> {
  const response = await apiClient.get<SymbolSummary>(
    `/engine/${encodeURIComponent(symbol)}/summary`,
  )

  return response.data
}