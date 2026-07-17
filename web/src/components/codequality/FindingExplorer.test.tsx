import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FindingExplorer } from './FindingExplorer'

const findings = [
  { id: 'one', dedupKey: 'one', title: 'First finding', description: '', cwe: '', severity: 'high' as const, kind: 'sca', status: 'open', priority: 1, scope: '', reachability: '' },
  { id: 'two', dedupKey: 'two', title: 'Second finding', description: '', cwe: '', severity: 'low' as const, kind: 'sca', status: 'open', priority: 1, scope: '', reachability: '' },
]

describe('FindingExplorer', () => {
  it('clears a selected finding when filters hide its row', () => {
    render(<FindingExplorer findings={findings as any} />)
    fireEvent.click(screen.getByRole('button', { name: /First finding/i }))
    expect(screen.getByRole('heading', { name: 'First finding' })).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Search findings'), { target: { value: 'Second' } })
    expect(screen.queryByRole('heading', { name: 'First finding' })).not.toBeInTheDocument()
    expect(screen.getByText('Select a finding to inspect its evidence and status.')).toBeInTheDocument()
  })
})
