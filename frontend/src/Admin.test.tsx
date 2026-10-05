import { render, screen, within } from '@testing-library/react'
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
      { code: 'GREEN', name: 'Green set', price: 4000, active: true },
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

  // fillBundleA builds "Green ×2 + Red ×1" in the rule form.
  const fillBundleA = async (user: ReturnType<typeof userEvent.setup>) => {
    await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))
    await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))
    await user.selectOptions(screen.getByLabelText('Bundle item 1'), 'GREEN')
    await user.type(screen.getByLabelText('Bundle quantity 1'), '2')
    await user.selectOptions(screen.getByLabelText('Bundle item 2'), 'RED')
    await user.type(screen.getByLabelText('Bundle quantity 2'), '1')
    await user.type(screen.getByLabelText('Percent'), '12')
  }

  it('lets an admin build a bundle rule', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createRule).mockImplementation(async (rule) => ({ ...rule, id: 7 }))
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Rule name'), 'Bundle A')
    await fillBundleA(user)
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(api.createRule).toHaveBeenCalledWith({
      name: 'Bundle A',
      bundle: [
        { itemCode: 'GREEN', qty: 2 },
        { itemCode: 'RED', qty: 1 },
      ],
      percent: 12,
      memberOnly: false,
      active: true,
    })
    expect(await screen.findByText('Bundle A')).toBeInTheDocument()
    expect(screen.getByText('2 × Green set + 1 × Red set')).toBeInTheDocument()
  })

  it('previews the bundle before saving', async () => {
    const user = userEvent.setup()
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await fillBundleA(user)

    expect(screen.getByText(/Preview/).parentElement).toHaveTextContent('2 × Green set + 1 × Red set')
    expect(screen.getByText(/Preview/).parentElement).toHaveTextContent('12% off')
  })

  it('removes a bundle row', async () => {
    const user = userEvent.setup()
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))
    await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))

    await user.click(screen.getByRole('button', { name: 'Remove bundle item 1' }))

    expect(screen.getAllByLabelText(/^Bundle item/)).toHaveLength(1)
  })

  it('shows the server error when a rule is rejected', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createRule).mockRejectedValue(new Error('invalid rule: percent must be between 1 and 100'))
    render(<Admin />)
    expect(await screen.findByText('RED')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Rule name'), 'bad')
    await user.type(screen.getByLabelText('Percent'), '5')
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('percent must be between 1 and 100')
    expect(screen.getByLabelText('Rule name')).toHaveValue('bad') // the form keeps what was typed
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
      bundle: [],
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
    const rule = { id: 3, name: 'Member', bundle: [], percent: 10, memberOnly: true, active: true }
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

  describe('form checks', () => {
    const submitItem = async (user: ReturnType<typeof userEvent.setup>, fields: { name?: string; price?: string }) => {
      expect(await screen.findByText('RED')).toBeInTheDocument()
      if (fields.name) await user.type(screen.getByLabelText('Item name'), fields.name)
      if (fields.price) await user.type(screen.getByLabelText('Price (THB)'), fields.price)
      await user.click(screen.getByRole('button', { name: 'Add item' }))
    }

    it('shows no errors on an untouched form', async () => {
      render(<Admin />)
      expect(await screen.findByText('RED')).toBeInTheDocument()

      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })

    it('blocks an empty item form and says what is missing', async () => {
      const user = userEvent.setup()
      render(<Admin />)

      await submitItem(user, {})

      expect(api.createItem).not.toHaveBeenCalled()
      expect(screen.getByText('Enter a name.')).toBeInTheDocument()
      expect(screen.getByText('Enter a price.')).toBeInTheDocument()
      expect(screen.getByLabelText('Item name')).toBeInvalid()
    })

    it.each(['12abc', '1e3', '-5', '0', '1.005', '1000000.01'])('blocks the price %j', async (price) => {
      const user = userEvent.setup()
      render(<Admin />)

      await submitItem(user, { name: 'Black set', price })

      expect(api.createItem).not.toHaveBeenCalled()
      expect(screen.getByLabelText('Price (THB)')).toBeInvalid()
    })

    it('clears a field error as soon as the field is fixed', async () => {
      const user = userEvent.setup()
      render(<Admin />)
      await submitItem(user, { name: 'Black set', price: 'abc' })
      expect(screen.getByLabelText('Price (THB)')).toBeInvalid()

      await user.clear(screen.getByLabelText('Price (THB)'))
      await user.type(screen.getByLabelText('Price (THB)'), '45')

      expect(screen.getByLabelText('Price (THB)')).toBeValid()
    })

    it('sends the item name without surrounding spaces', async () => {
      const user = userEvent.setup()
      vi.mocked(api.createItem).mockImplementation(async (item) => item)
      render(<Admin />)

      await submitItem(user, { name: '  Black set  ', price: '45' })

      expect(api.createItem).toHaveBeenCalledWith(expect.objectContaining({ name: 'Black set', price: 4500 }))
    })

    it('offers the next free colour and disables colours already used as codes', async () => {
      const user = userEvent.setup()
      vi.mocked(api.fetchAdminMenu).mockResolvedValue([{ code: '00CE7C', name: 'Mint', price: 1000, active: true }])
      render(<Admin />)
      expect(await screen.findByText('Mint')).toBeInTheDocument()

      expect(screen.getByLabelText('Item code')).not.toHaveTextContent('#00CE7C')
      await user.click(screen.getByLabelText('Item code'))
      expect(screen.getByRole('radio', { name: '#00CE7C' })).toBeDisabled()
    })

    it('blocks a rule with a bad percent, a missing item, a bad quantity and no name', async () => {
      const user = userEvent.setup()
      render(<Admin />)
      expect(await screen.findByText('RED')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))
      await user.type(screen.getByLabelText('Bundle quantity 1'), '2.5')
      await user.type(screen.getByLabelText('Percent'), '5.5')
      await user.click(screen.getByRole('button', { name: 'Add rule' }))

      expect(api.createRule).not.toHaveBeenCalled()
      expect(screen.getByLabelText('Rule name')).toBeInvalid()
      expect(screen.getByLabelText('Bundle item 1')).toBeInvalid()
      expect(screen.getByLabelText('Bundle quantity 1')).toBeInvalid()
      expect(screen.getByLabelText('Percent')).toBeInvalid()
    })

    it('stops the same item being picked twice in one bundle', async () => {
      const user = userEvent.setup()
      render(<Admin />)
      expect(await screen.findByText('RED')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))
      await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))
      await user.selectOptions(screen.getByLabelText('Bundle item 1'), 'GREEN')

      const second = screen.getByLabelText('Bundle item 2')
      expect(within(second).getByRole('option', { name: 'Green set' })).toBeDisabled()
      expect(within(second).getByRole('option', { name: 'Red set' })).toBeEnabled()
    })

    it('marks an item that is off the menu in the bundle picker', async () => {
      const user = userEvent.setup()
      vi.mocked(api.fetchAdminMenu).mockResolvedValue([{ code: 'RED', name: 'Red set', price: 5000, active: false }])
      render(<Admin />)
      expect(await screen.findByText('RED')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Add item to bundle' }))

      expect(screen.getByRole('option', { name: 'Red set (off the menu)' })).toBeInTheDocument()
    })

    it('saves once when the submit button is clicked twice', async () => {
      const user = userEvent.setup()
      let finish: (rule: api.Rule) => void = () => {}
      vi.mocked(api.createRule).mockReturnValue(new Promise((resolve) => (finish = resolve)))
      render(<Admin />)
      expect(await screen.findByText('RED')).toBeInTheDocument()
      await user.type(screen.getByLabelText('Rule name'), 'Lunch')
      await user.type(screen.getByLabelText('Percent'), '5')

      const submit = screen.getByRole('button', { name: 'Add rule' })
      await user.dblClick(submit)

      expect(api.createRule).toHaveBeenCalledTimes(1)
      expect(screen.getAllByRole('button', { name: 'Saving…' })).toHaveLength(2)
      expect(screen.getByRole('switch', { name: 'Red set active' })).toBeDisabled()
      finish({ id: 9, name: 'Lunch', bundle: [], percent: 5, memberOnly: false, active: true })
      expect(await screen.findByText('Lunch')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Add rule' })).toBeEnabled()
    })
  })
})
