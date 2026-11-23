/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useForm } from 'react-hook-form'
import { describe, expect, test, vi } from 'vitest'

import { Form } from '@/components/ui/form'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  type ChannelFormValues,
} from '../../lib/channel-form'
import { ChannelUserRulesSetting } from '../channel-user-rules-setting'

const SAVED_RULES = [
  '[',
  '  {"user_id":761},',
  '  {"user_id":822,"models":["claude-haiku-5-5"],"non_stream_only":true},',
  '  {"user_id":822,"rpm":30}',
  ']',
].join('\n')

// The setting lives inside the channel form, so a fixture form stands in for
// it and reports the field text the channel would save.
function ChannelForm(props: {
  userRules: string
  disabled?: boolean
  onSave: (userRules: string) => void
}) {
  const form = useForm<ChannelFormValues>({
    defaultValues: {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      models: 'claude-haiku-5-5,claude-opus-5-5',
      user_rules: props.userRules,
    },
  })
  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSave(values.user_rules ?? '')
        )}
      >
        <ChannelUserRulesSetting disabled={props.disabled} />
        <button type='submit'>Save channel</button>
      </form>
    </Form>
  )
}

describe('channel user rules setting', () => {
  test('shows each saved rule as a table row', () => {
    render(<ChannelForm userRules={SAVED_RULES} onSave={vi.fn()} />)

    const rows = screen.getAllByRole('row').slice(1)

    expect(
      rows.map((row) =>
        within(row)
          .getAllByRole('cell')
          .slice(0, 4)
          .map((cell) => cell.textContent)
      )
    ).toEqual([
      ['761', 'All models', 'All requests', 'Always block'],
      ['822', 'claude-haiku-5-5', 'Non-streaming only', 'Always block'],
      ['822', 'All models', 'All requests', '30 per minute'],
    ])
  })

  test('adding a rule in the dialog writes it to the field without saving the channel', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<ChannelForm userRules='' onSave={onSave} />)
    expect(screen.getByText('No user rules yet.')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    const dialog = await screen.findByRole('dialog', { name: 'Add rule' })
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'User ID' }),
      '822'
    )
    await user.click(
      within(dialog).getByRole('switch', {
        name: 'Non-streaming requests only',
      })
    )
    await user.type(
      within(dialog).getByRole('spinbutton', { name: 'Allowed per minute' }),
      '30'
    )
    await user.click(
      within(dialog).getByRole('combobox', { name: 'All models' })
    )
    await user.click(
      await screen.findByRole('option', { name: 'claude-haiku-5-5' })
    )
    await user.keyboard('{Escape}')
    await user.click(within(dialog).getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(onSave).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Save channel' }))
    expect(onSave).toHaveBeenCalledWith(
      '[\n  {"user_id":822,"models":["claude-haiku-5-5"],"non_stream_only":true,"rpm":30}\n]'
    )
  })

  test('editing a row replaces only that rule', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<ChannelForm userRules={SAVED_RULES} onSave={onSave} />)

    await user.click(
      within(screen.getByRole('row', { name: /30 per minute/ })).getByRole(
        'button',
        { name: 'Edit' }
      )
    )
    const dialog = await screen.findByRole('dialog', { name: 'Edit rule' })
    const rpm = within(dialog).getByRole('spinbutton', {
      name: 'Allowed per minute',
    })
    expect(rpm).toHaveValue(30)
    await user.clear(rpm)
    await user.type(rpm, '5')
    await user.click(within(dialog).getByRole('button', { name: 'Update' }))
    await user.click(screen.getByRole('button', { name: 'Save channel' }))

    expect(onSave).toHaveBeenCalledWith(
      [
        '[',
        '  {"user_id":761},',
        '  {"user_id":822,"models":["claude-haiku-5-5"],"non_stream_only":true},',
        '  {"user_id":822,"rpm":5}',
        ']',
      ].join('\n')
    )
  })

  test('deleting a row removes only that rule', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<ChannelForm userRules={SAVED_RULES} onSave={onSave} />)

    await user.click(
      within(screen.getByRole('row', { name: /761/ })).getByRole('button', {
        name: 'Open menu',
      })
    )
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    await user.click(screen.getByRole('button', { name: 'Save channel' }))

    expect(onSave).toHaveBeenCalledWith(
      [
        '[',
        '  {"user_id":822,"models":["claude-haiku-5-5"],"non_stream_only":true},',
        '  {"user_id":822,"rpm":30}',
        ']',
      ].join('\n')
    )
  })

  test('a rule without a user ID stays in the dialog with an error', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()
    render(<ChannelForm userRules='' onSave={onSave} />)

    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    const dialog = await screen.findByRole('dialog', { name: 'Add rule' })
    await user.click(within(dialog).getByRole('button', { name: 'Add' }))

    expect(
      await within(dialog).findByText('Enter a positive user ID')
    ).toBeInTheDocument()
    expect(
      within(dialog).getByRole('spinbutton', { name: 'User ID' })
    ).toHaveAttribute('aria-invalid', 'true')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await user.click(screen.getByRole('button', { name: 'Save channel' }))
    expect(onSave).toHaveBeenCalledWith('')
  })

  test('invalid rules ask to be fixed in JSON mode and keep their text', async () => {
    const user = userEvent.setup()
    render(<ChannelForm userRules='[{"user_id": 0}]' onSave={vi.fn()} />)

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Each user rule needs a positive user_id'
    )
    expect(
      screen.queryByRole('button', { name: 'Add rule' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Switch to JSON' }))

    expect(screen.getByRole('textbox')).toHaveValue('[{"user_id": 0}]')
  })

  test('rules typed in JSON mode show up as rows after switching back', async () => {
    const user = userEvent.setup()
    render(<ChannelForm userRules='' onSave={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: 'Switch to JSON' }))
    await user.click(screen.getByRole('textbox'))
    await user.paste('[{"user_id": 761, "rpm": 3}]')
    await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))

    expect(
      screen.getByRole('row', { name: /761.*3 per minute/ })
    ).toBeInTheDocument()
  })

  test('a locked channel shows the rules but cannot change them', () => {
    render(<ChannelForm userRules={SAVED_RULES} disabled onSave={vi.fn()} />)

    expect(screen.getByRole('button', { name: 'Add rule' })).toBeDisabled()
    for (const edit of screen.getAllByRole('button', { name: 'Edit' })) {
      expect(edit).toBeDisabled()
    }
  })
})
