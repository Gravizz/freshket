import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import * as api from './api'

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  fetchMenu: vi.fn(),
  calculate: vi.fn(),
}))

describe('App', () => {
  beforeEach(() => {
    vi.mocked(api.fetchMenu).mockResolvedValue([
      { code: 'RED', name: 'Red set', price: 5000 },
      { code: 'GREEN', name: 'Green set', price: 4000 },
    ])
    vi.mocked(api.calculate).mockResolvedValue({ subtotal: 0, discounts: [], total: 0 })
  })

  it('sends the selected items and member flag, then renders the breakdown', async () => {
    const user = userEvent.setup()
    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Add Red set' }))
    vi.mocked(api.calculate).mockResolvedValue({
      subtotal: 5000,
      discounts: [{ label: 'Member 10%', amount: 500 }],
      total: 4500,
    })
    await user.click(screen.getByRole('checkbox', { name: 'Member card' }))

    await waitFor(() =>
      expect(api.calculate).toHaveBeenLastCalledWith([{ code: 'RED', qty: 1 }], true),
    )
    expect(await screen.findByText('฿45.00')).toBeInTheDocument()
    expect(screen.getByText('Member 10%')).toBeInTheDocument()
  })
})
