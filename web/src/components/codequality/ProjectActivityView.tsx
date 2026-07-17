import { Activity, CheckCircle2, XCircle } from 'lucide-react'
import { useState } from 'react'
import type { ProjectAnalysis } from '../../lib/types'
import { Button, Card, EmptyState, Pill } from '../ui'

type Mode = 'overall' | 'new'
type Props = { analyses: ProjectAnalysis[]; hasOlder?: boolean; loadingOlder?: boolean; onLoadOlder?: () => void }

export function ProjectActivityView({ analyses, hasOlder = false, loadingOlder = false, onLoadOlder }: Props) {
  const [mode, setMode] = useState<Mode>('overall')
  if (analyses.length === 0) {
    return <Card title="Activity"><EmptyState icon={Activity} title="No analysis history yet" hint="Each successful analysis will appear here." /></Card>
  }
  const chronological = [...analyses].reverse()
  const values = (select: (analysis: ProjectAnalysis) => number) => chronological.map((analysis) => ({ label: formatDate(analysis.createdAt), value: select(analysis) }))
  const ratings = (quality: 'security' | 'reliability' | 'maintainability') => values((analysis) => gradeValue(mode === 'overall' ? analysis.rating[quality] : analysis.newCode.rating[quality]))
  const coverage = chronological.filter((analysis) => analysis.coverage !== null).map((analysis) => ({ label: formatDate(analysis.createdAt), value: analysis.coverage && analysis.coverage.totalLines > 0 ? 100 * analysis.coverage.coveredLines / analysis.coverage.totalLines : 0 }))
  return <section className="space-y-6" aria-label="Project activity">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div><h2 className="text-xl font-semibold">Activity</h2><p className="mt-1 text-sm text-mutedfg">New Code is compared with the previous successful analysis. Showing {analyses.length} most recent analyses.</p></div>
      <div className="flex rounded-lg border border-border bg-elevated p-1" aria-label="Activity scope">
        <button type="button" aria-pressed={mode === 'overall'} onClick={() => setMode('overall')} className={`rounded-md px-3 py-1.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 ${mode === 'overall' ? 'bg-card text-foreground shadow-sm' : 'text-mutedfg'}`}>Overall</button>
        <button type="button" aria-pressed={mode === 'new'} onClick={() => setMode('new')} className={`rounded-md px-3 py-1.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 ${mode === 'new' ? 'bg-card text-foreground shadow-sm' : 'text-mutedfg'}`}>New Code</button>
      </div>
    </div>
    <Card title="Trends">
      <div className="grid gap-4 lg:grid-cols-2">
        <Trend title={mode === 'overall' ? 'Total issues' : 'New issues'} values={values((analysis) => mode === 'overall' ? analysis.issues.total : analysis.newCode.counts.total)} />
        <Trend title={mode === 'overall' ? 'Security rating' : 'New Code security rating'} values={ratings('security')} grade />
        <Trend title={mode === 'overall' ? 'Reliability rating' : 'New Code reliability rating'} values={ratings('reliability')} grade />
        <Trend title={mode === 'overall' ? 'Maintainability rating' : 'New Code maintainability rating'} values={ratings('maintainability')} grade />
        {mode === 'overall' ? <>
          <Trend title="Duplication density" subtitle="Overall metric; change is since previous analysis." values={values((analysis) => analysis.duplication.totalLines ? 100 * analysis.duplication.duplicatedLines / analysis.duplication.totalLines : 0)} suffix="%" />
          {coverage.length > 0 && <Trend title="Line coverage" values={coverage} suffix="%" />}
        </> : <>
          <Trend title="New critical" values={values((analysis) => analysis.newCode.counts.bySeverity.critical ?? 0)} />
          <Trend title="New high" values={values((analysis) => analysis.newCode.counts.bySeverity.high ?? 0)} />
        </>}
      </div>
      {mode === 'overall' && coverage.length === 0 && <p className="mt-4 text-sm text-mutedfg">Line coverage is unavailable because no coverage artifact was supplied.</p>}
    </Card>
    <Card title="Analysis timeline">
      <ol className="divide-y divide-border">
        {analyses.map((analysis, index) => <li key={analysis.id} className="py-4 first:pt-0 last:pb-0">
          <div className="flex flex-wrap items-start justify-between gap-3"><div><p className="font-medium">{formatDate(analysis.createdAt)} {index === analyses.length - 1 && !hasOlder && <span className="text-sm font-normal text-mutedfg">· first analysis</span>}</p><p className="mt-1 font-mono text-xs text-mutedfg">{analysis.sourceRef || 'source ref unavailable'}{analysis.sourceCommit ? ` · ${analysis.sourceCommit.slice(0, 12)}` : ''}</p></div><GatePill passed={analysis.gate.passed} /></div>
          {mode === 'overall' ? <dl className="mt-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4"><Metric label="Issues" value={analysis.issues.total} />{analysis.delta && <><Metric label="Critical change since previous" value={signed(analysis.delta.issues.bySeverity.critical ?? 0)} /><Metric label="High change since previous" value={signed(analysis.delta.issues.bySeverity.high ?? 0)} /><Metric label="Duplication change since previous" value={`${signed(analysis.delta.measures.duplication_density ?? 0)}%`} /></>}</dl> : <dl className="mt-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-3"><Metric label="New issues" value={analysis.newCode.counts.total} /><Metric label="New critical" value={analysis.newCode.counts.bySeverity.critical ?? 0} /><Metric label="New high" value={analysis.newCode.counts.bySeverity.high ?? 0} /></dl>}
        </li>)}
      </ol>
      {hasOlder && onLoadOlder && <div className="mt-4 border-t border-border pt-4 text-center"><Button variant="secondary" loading={loadingOlder} disabled={loadingOlder} onClick={onLoadOlder}>Load older</Button></div>}
    </Card>
  </section>
}

function Trend({ title, subtitle, values, suffix = '', grade = false }: { title: string; subtitle?: string; values: { label: string; value: number }[]; suffix?: string; grade?: boolean }) {
  const width = 280; const height = 92
  const max = Math.max(1, ...values.map((point) => point.value)); const min = Math.min(0, ...values.map((point) => point.value)); const range = Math.max(1, max - min)
  const points = values.map((point, index) => `${values.length === 1 ? width / 2 : index * width / (values.length - 1)},${height - ((point.value - min) / range) * (height - 12) - 6}`).join(' ')
  const summary = values.map((point) => `${point.label}: ${grade ? gradeLabel(point.value) : point.value.toFixed(1)}${suffix}`).join(', ')
  return <div className="rounded-lg border border-border bg-bg p-4"><h3 className="font-medium">{title}</h3>{subtitle && <p className="mt-1 text-xs text-mutedfg">{subtitle}</p>}<svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`${title} trend: ${summary}`} className="mt-3 h-24 w-full overflow-visible"><title>{title} trend</title><polyline fill="none" stroke="currentColor" strokeWidth="3" points={points} className="text-brand" /></svg><ul className="mt-2 space-y-1 text-xs text-mutedfg">{values.map((point) => <li key={point.label} className="flex justify-between gap-3"><span>{point.label}</span><span className="font-mono text-foreground">{grade ? gradeLabel(point.value) : point.value.toFixed(1)}{suffix}</span></li>)}</ul></div>
}

function GatePill({ passed }: { passed: boolean }) { return <Pill className={passed ? 'bg-low/15 text-low' : 'bg-critical/15 text-critical'}>{passed ? <CheckCircle2 className="size-3" aria-hidden="true" /> : <XCircle className="size-3" aria-hidden="true" />}{passed ? 'Gate passed' : 'Gate failed'}</Pill> }
function Metric({ label, value }: { label: string; value: string | number }) { return <div><dt className="text-[10px] font-semibold uppercase tracking-[0.12em] text-subtlefg">{label}</dt><dd className="mt-1 font-mono text-lg tabular-nums">{value}</dd></div> }
function signed(value: number) { return value > 0 ? `+${value}` : String(value) }
function gradeValue(grade: string) { return ({ A: 1, B: 2, C: 3, D: 4, E: 5 } as Record<string, number>)[grade] ?? 5 }
function gradeLabel(value: number) { return ['A', 'B', 'C', 'D', 'E'][Math.min(5, Math.max(1, Math.round(value))) - 1] }
function formatDate(value: string) { return new Date(value).toLocaleString() }
