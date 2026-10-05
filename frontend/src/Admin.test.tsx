import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Admin from './Admin'
import * as api from './api'

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  fetchAdminMenu: vi.fn(),
  fetchRules: vi.fn(),
  createItem: vi.fn(),
  updateItem: vi.fn(),
  createRule: vi.fn(),
  updateRule: vi.fn(),
}))

describe('Admin', () => {
  beforeEach(() => {
    vi.mocked(api.fetchAdminMenu).mockResolvedValue([
      { code: 'RED', name: 'Red set', price: 5000, active: true },
    ])
    vi.mocked(api.fetchRules).mockResolvedValue([])
  })

  it('lets an admin add a menu item coded by a honeycomb colour and priced in baht', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createItem).mockImplementation(async (item) => item)
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await user.click(screen.getByLabelText('Item code'))
    await user.click(screen.getByRole('radio', { name: '#E05252' }))
    await user.type(screen.getByLabelText('Item name'), 'Black set')
    await user.type(screen.getByLabelText('Price (THB)'), '45.50')
    await user.click(screen.getByRole('button', { name: 'Add item' }))

    expect(api.createItem).toHaveBeenCalledWith({
      code: 'E05252',
      name: 'Black set',
      price: 4550,
      active: true,
    })
    expect(await screen.findByText('#E05252')).toBeInTheDocument()
  })

  it('lets an admin add a discount rule for an item', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createRule).mockImplementation(async (rule) => ({ ...rule, id: 7 }))
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Rule name'), 'triple')
    await user.selectOptions(screen.getByLabelText('Rule item'), 'RED')
    await user.type(screen.getByLabelText('Group size'), '3')
    await user.type(screen.getByLabelText('Percent'), '10')
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(api.createRule).toHaveBeenCalledWith({
      name: 'triple',
      itemCode: 'RED',
      groupSize: 3,
      percent: 10,
      memberOnly: false,
      active: true,
    })
    expect(await screen.findByText('triple')).toBeInTheDocument()
  })

  it('adds a whole-order rule for members when no item is chosen', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createRule).mockImplementation(async (rule) => ({ ...rule, id: 8 }))
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Rule name'), 'Gold member')
    await user.type(screen.getByLabelText('Percent'), '15')
    await user.click(screen.getByLabelText('Members only'))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(api.createRule).toHaveBeenCalledWith({
      name: 'Gold member',
      itemCode: '',
      groupSize: 0,
      percent: 15,
      memberOnly: true,
      active: true,
    })
  })

  it('shows the server error when adding fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createItem).mockRejectedValue(new Error('duplicate item code: 00CE7C'))
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Item name'), 'Another green')
    await user.type(screen.getByLabelText('Price (THB)'), '10')
    await user.click(screen.getByRole('button', { name: 'Add item' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('duplicate item code: 00CE7C')
  })

  it('takes an item off the menu with its active switch', async () => {
    const user = userEvent.setup()
    vi.mocked(api.updateItem).mockImplementation(async (item) => item)
    render(<Admin />)

    const toggle = await screen.findByRole('switch', { name: 'Red set active' })
    expect(toggle).toBeChecked()
    await user.click(toggle)

    expect(api.updateItem).toHaveBeenCalledWith({ code: 'RED', name: 'Red set', price: 5000, active: false })
    expect(await screen.findByRole('switch', { name: 'Red set active' })).not.toBeChecked()
    expect(screen.getByRole('status')).toHaveTextContent('Red set')
  })

  it('pauses a discount rule with its active switch', async () => {
    const user = userEvent.setup()
    const rule = { id: 3, name: 'Member', itemCode: '', groupSize: 0, percent: 10, memberOnly: true, active: true }
    vi.mocked(api.fetchRules).mockResolvedValue([rule])
    vi.mocked(api.updateRule).mockImplementation(async (r) => r)
    render(<Admin />)

    await user.click(await screen.findByRole('switch', { name: 'Member active' }))

    expect(api.updateRule).toHaveBeenCalledWith({ ...rule, active: false })
    expect(await screen.findByRole('switch', { name: 'Member active' })).not.toBeChecked()
  })

  it('shows an error when the lists cannot be loaded', async () => {
    vi.mocked(api.fetchAdminMenu).mockRejectedValue(new Error('backend is down'))
    render(<Admin />)

    expect(await screen.findByRole('alert')).toHaveTextContent('backend is down')
  })
})
