import { api, call } from './client'
import { apiBaseUrl, authHeader } from './auth'
import type { Receipt, ReceiptCreated } from './types'

export const receiptsApi = {
  // Uploads the QR url + receipt photo as multipart/form-data (raw fetch: the
  // body is a file, not JSON). The backend scrapes and parses asynchronously.
  create: async (qr_url: string, photo: Blob): Promise<ReceiptCreated> => {
    const form = new FormData()
    form.set('qr_url', qr_url)
    form.set('photo', photo, 'receipt.jpg')

    const header = authHeader()
    const res = await fetch(`${apiBaseUrl}/receipts`, {
      method: 'POST',
      headers: header ? { Authorization: header } : {},
      body: form,
    })
    if (!res.ok) {
      let msg = `HTTP ${res.status}`
      try {
        const body = (await res.json()) as { error?: string }
        if (body?.error) msg = body.error
      } catch {
        // non-JSON error body; keep the status message
      }
      throw new Error(msg)
    }

    return res.json() as Promise<ReceiptCreated>
  },

  get: (id: string): Promise<Receipt> => call(api.GET('/receipts/{id}', { params: { path: { id } } })),

  list: (page = 1, limit = 20): Promise<Receipt[]> =>
    call(api.GET('/receipts', { params: { query: { page, limit } } })).then((d) => d.receipts),

  linkTransaction: (id: string, transaction_id: string): Promise<Receipt> =>
    call(api.PUT('/receipts/{id}/transaction', { params: { path: { id } }, body: { transaction_id } })),

  unlinkTransaction: (id: string): Promise<Receipt> =>
    call(api.DELETE('/receipts/{id}/transaction', { params: { path: { id } } })),
}
