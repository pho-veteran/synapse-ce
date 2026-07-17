import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FindingExplorer } from './FindingExplorer'

const findings = [
  { id: 'one', dedupKey: 'one', title: 'First finding', description: '', cwe: '', severity: 'high' as const, kind: 'sca', status: 'open', priority: 1, scope: '', reachability: '' },
  { id: 'two', dedupKey: 'two', title: 'Second finding', description: '', cwe: '', severity: 'low' as const, kind: 'sca', status: 'open', priority: 1, scope: '', reachability: '' },
]

const finding = (index: number) => ({ id: `finding-${index}`, dedupKey: `finding-${index}`, title: `Finding ${index}`, description: '', cwe: '', severity: 'high' as const, kind: 'sca', status: 'open', priority: 1, scope: '', reachability: '' })

describe('FindingExplorer', () => {
  it('clears a selected finding when filters hide its row', () => {
    render(<FindingExplorer findings={findings as any} />)
    fireEvent.click(screen.getByRole('button', { name: /First finding/i }))
    expect(screen.getByRole('heading', { name: 'First finding' })).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Search findings'), { target: { value: 'Second' } })
    expect(screen.queryByRole('heading', { name: 'First finding' })).not.toBeInTheDocument()
    expect(screen.getByText('Select a finding to inspect its evidence and status.')).toBeInTheDocument()
  })

  it('uses the id and dedup key to identify empty-id findings', () => {
    const emptyID = [{ ...findings[0], id: undefined, dedupKey: 'first' }, { ...findings[1], id: undefined, dedupKey: 'second' }]
    render(<FindingExplorer findings={emptyID as any} />)
    fireEvent.click(screen.getByRole('button', { name: /First finding/i }))
    expect(screen.getByRole('button', { name: /First finding/i })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Second finding/i })).toHaveAttribute('aria-pressed', 'false')
  })

  it('refreshes selected details when the finding data changes', () => {
    const { rerender } = render(<FindingExplorer findings={findings as any} />)
    fireEvent.click(screen.getByRole('button', { name: /First finding/i }))
    rerender(<FindingExplorer findings={[{ ...findings[0], title: 'Updated finding', description: 'Updated detail' }, findings[1]] as any} />)
    expect(screen.getByRole('heading', { name: 'Updated finding' })).toBeInTheDocument()
    expect(screen.getByText('Updated detail')).toBeInTheDocument()
  })

  it('renders 50 findings at a time', () => {
    render(<FindingExplorer findings={Array.from({ length: 51 }, (_, index) => finding(index)) as any} />)
    expect(screen.getByRole('button', { name: /Finding 49/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Finding 50/ })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Load more findings' }))
    expect(screen.getByRole('button', { name: /Finding 50/ })).toBeInTheDocument()
  })
})
