import type {
  SubmitOrderRequest,
  SubmitOrderResponse,
} from '../types/order'
import { apiClient } from './client'

async function submitOrderRequest(
  request: SubmitOrderRequest,
): Promise<SubmitOrderResponse> {
  const response = await apiClient.post<SubmitOrderResponse>(
    '/orders',
    request,
  )

  return response.data
}

export {
    submitOrderRequest
}