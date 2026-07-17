import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ProjectActivityView } from './ProjectActivityView'

const analysis = {
  id: 'a1', createdAt: '2026-07-16T12:00:00Z', sourceRef: 'main', sourceCommit: 'abcdef1234567890',
  gate: { passed: false, results: [] }, issues: { total: 2, byKind: {}, bySeverity: { critical: 1 }, byStatus: {} },
  newCode: { previousId: '', counts: { total: 2, byKind: {}, bySeverity: { critical: 1 }, byStatus: {} }, rating: { security: 'E' as const, reliability: 'A' as const, maintainability: 'A' as const, techDebtMinutes: 0, debtRatioPct: 0, linesOfCode: 10 } },
  delta: null, measures: {}, coverage: null,
  duplication: { blocks: [], duplicatedLines: 0, totalLines: 0, files: 0 }, rating: { security: 'E' as const, reliability: 'A' as const, maintainability: 'A' as const, techDebtMinutes: 0, debtRatioPct: 0, linesOfCode: 10 },
}

describe('ProjectActivityView', () => {
  it('labels the first baseline and does not fabricate coverage', () => {
    render(<ProjectActivityView analyses={[analysis]} />)
    expect(screen.getByText(/first analysis/i)).toBeInTheDocument()
    expect(screen.getByText(/Line coverage is unavailable/i)).toBeInTheDocument()
    expect(screen.getByText('Gate failed')).toBeInTheDocument()
    expect(screen.queryByText(/Critical change since previous/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Duplication change since previous/i)).not.toBeInTheDocument()
  })

  it('switches to scoped New Code trends with an accessible toggle', () => {
    render(<ProjectActivityView analyses={[analysis]} />)
    expect(screen.getByText('Reliability rating')).toBeInTheDocument()
    expect(screen.getByText('Maintainability rating')).toBeInTheDocument()
    const toggle = screen.getByRole('button', { name: 'New Code' })
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getAllByText('New issues')).not.toHaveLength(0)
    expect(screen.getByText('New Code reliability rating')).toBeInTheDocument()
    expect(screen.getByText('New Code maintainability rating')).toBeInTheDocument()
    expect(screen.queryByText(/Duplication change since previous/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Line coverage is unavailable/i)).not.toBeInTheDocument()
  })

  it('loads older history on demand', () => {
    const onLoadOlder = vi.fn()
    render(<ProjectActivityView analyses={[analysis]} hasOlder onLoadOlder={onLoadOlder} />)
    fireEvent.click(screen.getByRole('button', { name: 'Load older' }))
    expect(onLoadOlder).toHaveBeenCalledOnce()
    expect(screen.queryByText(/first analysis/i)).not.toBeInTheDocument()
  })
})
