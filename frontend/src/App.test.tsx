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

  it('lists two discounts that share a label without a duplicate-key warning', async () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(api.calculate).mockResolvedValue({
      subtotal: 10000,
      discounts: [
        { label: 'Welcome 5%', amount: 100 },
        { label: 'Welcome 5%', amount: 200 },
      ],
      total: 9700,
    })
    render(<App />)

    expect(await screen.findByText('−฿1.00')).toBeInTheDocument()
    expect(screen.getByText('−฿2.00')).toBeInTheDocument()
    expect(errors.mock.calls.flat().join(' ')).not.toContain('same key')
    errors.mockRestore()
  })

  it('shows the menu error with a retry that recovers', async () => {
    const user = userEvent.setup()
    vi.mocked(api.fetchMenu).mockRejectedValueOnce(new Error('Cannot reach the server.'))
    render(<App />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot reach the server.')
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByRole('button', { name: 'Add Red set' })).toBeInTheDocument()
    expect(screen.queryByText(/Cannot reach/)).not.toBeInTheDocument()
  })

  it('says so when no sets are on the menu', async () => {
    vi.mocked(api.fetchMenu).mockResolvedValue([])
    render(<App />)

    expect(await screen.findByText(/No sets are on the menu/)).toBeInTheDocument()
  })

  it('drops a set taken off the menu from the basket instead of failing forever', async () => {
    const user = userEvent.setup()
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Add Red set' }))
    await screen.findByText('Order summary')

    // Red is taken off the menu elsewhere; the next calculation is rejected.
    vi.mocked(api.fetchMenu).mockResolvedValue([{ code: 'GREEN', name: 'Green set', price: 4000 }])
    vi.mocked(api.calculate).mockRejectedValueOnce(new Error('unknown item: "RED"'))
    await user.click(screen.getByRole('button', { name: 'Add Red set' }))

    expect(await screen.findByRole('status')).toHaveTextContent('no longer on the menu')
    expect(screen.queryByRole('button', { name: 'Add Red set' })).not.toBeInTheDocument()
    await waitFor(() => expect(api.calculate).toHaveBeenLastCalledWith([], false))
  })

  it('shows no stale total next to a calculation error', async () => {
    const user = userEvent.setup()
    vi.mocked(api.calculate).mockResolvedValue({ subtotal: 5000, discounts: [{ label: 'Promo', amount: 500 }], total: 4500 })
    render(<App />)
    await user.click(await screen.findByRole('button', { name: 'Add Red set' }))
    expect(await screen.findByText('฿45.00')).toBeInTheDocument()

    vi.mocked(api.calculate).mockRejectedValue(new Error('Cannot reach the server.'))
    await user.click(screen.getByRole('button', { name: 'Add Red set' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Cannot reach the server.')
    expect(screen.queryByText('฿45.00')).not.toBeInTheDocument()
  })
})
