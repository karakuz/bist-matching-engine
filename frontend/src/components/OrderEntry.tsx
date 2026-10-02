import { useState, type FormEvent } from 'react'
import axios from 'axios'
import {
  Alert,
  Button,
  NumberInput,
  Paper,
  SegmentedControl,
  Stack,
  Text,
} from '@mantine/core'
import { submitOrderRequest } from '../api/orders'
import type { OrderSide, SubmitOrderRequest } from '../types/order'

type OrderEntryProps = {
  symbol: string
  tickSize: number
}

const PARTICIPANT_ID = 2

function getSubmissionError(error: unknown) {
  if (axios.isAxiosError(error)) {
    const responseMessage = error.response?.data?.message

    if (typeof responseMessage === 'string') {
      return responseMessage
    }
  }

  return error instanceof Error ? error.message : 'Could not submit order'
}

export function OrderEntry({ symbol, tickSize }: OrderEntryProps) {
  const [price, setPrice] = useState<number | string>('')
  const [quantity, setQuantity] = useState<number | string>('')
  const [side, setSide] = useState<OrderSide>('BUY')
  const [submissionMessage, setSubmissionMessage] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function submitOrder(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmissionMessage('')
    setErrorMessage('')

    const numericPrice = Number(price)
    const numericQuantity = Number(quantity)

    if (!Number.isInteger(numericPrice) || numericPrice <= 0) {
      setErrorMessage('Enter a positive whole-number price.')
      return
    }

    if (!Number.isInteger(numericQuantity) || numericQuantity <= 0) {
      setErrorMessage('Enter a positive whole-number quantity.')
      return
    }

    if (numericPrice % tickSize !== 0) {
      setErrorMessage(`Price must be a multiple of the ${tickSize} tick size.`)
      return
    }

    const request: SubmitOrderRequest = {
      participantId: PARTICIPANT_ID,
      symbol,
      side,
      price: numericPrice,
      quantity: numericQuantity,
    }

    setIsSubmitting(true)

    try {
      const response = await submitOrderRequest(request)
      setSubmissionMessage(response.message || 'Order queued.')
      setQuantity('')
    } catch (error) {
      setErrorMessage(getSubmissionError(error))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Paper className="panel-card order-entry" radius="md" p="lg" withBorder>
      <Text className="eyebrow">PLACE AN ORDER</Text>
      <Text className="panel-title" mb="lg">Limit order</Text>

      <form onSubmit={submitOrder}>
        <Stack gap="md">
          <SegmentedControl
            fullWidth
            aria-label="Order side"
            value={side}
            onChange={(value) => setSide(value as OrderSide)}
            data={[
              { label: 'Buy', value: 'BUY' },
              { label: 'Sell', value: 'SELL' },
            ]}
          />
          <NumberInput
            label="Limit price"
            description={`Price in kuruş · tick ${tickSize}`}
            value={price}
            onChange={setPrice}
            min={1}
            step={tickSize}
            decimalScale={0}
            thousandSeparator="."
            decimalSeparator={','}
            required
          />
          <NumberInput
            label="Quantity"
            value={quantity}
            onChange={setQuantity}
            min={1}
            decimalScale={0}
            thousandSeparator="."
            decimalSeparator={','}
            required
          />
          {errorMessage && <Alert color="red">{errorMessage}</Alert>}
          {submissionMessage && (
            <Alert color="teal">{submissionMessage}</Alert>
          )}
          <Button
            type="submit"
            loading={isSubmitting}
            color={side === 'BUY' ? 'teal' : 'red'}
          >
            Submit {side.toLowerCase()} order
          </Button>
          <Text c="dimmed" size="xs" ta="center">
            Orders are submitted to the matching engine for processing.
          </Text>
        </Stack>
      </form>
    </Paper>
  )
}
