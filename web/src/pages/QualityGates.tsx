import { Plus, ShieldCheck, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { QualityGate, QualityGateCondition } from '../lib/types'
import { Button, Card, EmptyState, ErrorState, Field, Input, Select, Spinner } from '../components/ui'

const metrics = ['new_critical', 'new_high', 'new_medium', 'new_secret', 'new_vulnerability', 'new_issues', 'total_critical', 'coverage', 'duplication_density', 'security_rating', 'reliability_rating', 'maintainability_rating']
const operators: QualityGateCondition['op'][] = ['<=', '>=', '==', '<', '>']

export function QualityGates() {
  const [gates, setGates] = useState<QualityGate[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  function load() {
    setError(null)
    api.listQualityGates().then(setGates).catch((e) => setError(e instanceof Error ? e.message : 'Failed to load quality gates'))
  }
  useEffect(load, [])

  async function remove(key: string) {
    setError(null)
    try { await api.deleteQualityGate(key); load() } catch (e) { setError(e instanceof Error ? e.message : 'Failed to delete quality gate') }
  }

  return <div className="mx-auto max-w-6xl animate-fade-in">
    <header className="bg-hero mb-6 flex flex-wrap items-center justify-between gap-4 rounded-xl border border-border p-6">
      <div><div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-branddim"><ShieldCheck className="size-4" aria-hidden="true" />Quality gates</div><h1 className="text-3xl font-bold tracking-tight">Quality Gates</h1><p className="mt-1.5 max-w-xl text-sm text-mutedfg">Set measurable pass/fail conditions, then assign a gate to a code-quality project.</p></div>
      <Button variant="brand" onClick={() => setCreating((value) => !value)}>{creating ? 'Cancel' : <><Plus className="size-4" aria-hidden="true" /> New gate</>}</Button>
    </header>
    {creating && <CreateGate onCreated={() => { setCreating(false); load() }} />}
    {error && <div className="mb-6"><ErrorState message={error} /><Button className="mt-3" variant="secondary" onClick={load}>Retry</Button></div>}
    {!gates && !error && <Spinner label="Loading quality gates…" />}
    {gates?.length === 0 && <EmptyState icon={ShieldCheck} title="No quality gates" hint="Create a custom gate or use the built-in default." />}
    {gates && gates.length > 0 && <div className="grid gap-4 md:grid-cols-2">{gates.map((gate) => <Card key={gate.key} title={gate.name} className="relative"><p className="font-mono text-xs text-mutedfg">{gate.key}{gate.builtIn ? ' · built-in' : ''}</p><ul className="mt-4 space-y-2 text-sm">{gate.conditions.map((condition, index) => <li key={index} className="rounded bg-bg px-3 py-2 font-mono text-xs">{condition.metric} {condition.op} {condition.threshold}</li>)}</ul>{!gate.builtIn && <Button className="mt-4" variant="secondary" onClick={() => remove(gate.key)}><Trash2 className="size-4" aria-hidden="true" /> Delete</Button>}</Card>)}</div>}
  </div>
}

function CreateGate({ onCreated }: { onCreated: () => void }) {
  const [key, setKey] = useState('')
  const [name, setName] = useState('')
  const [metric, setMetric] = useState('new_high')
  const [op, setOp] = useState<QualityGateCondition['op']>('<=')
  const [threshold, setThreshold] = useState('0')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  async function submit(event: React.FormEvent) {
    event.preventDefault(); setSaving(true); setError(null)
    try {
      await api.createQualityGate({ key: key.trim(), name: name.trim(), conditions: [{ metric, op, threshold: Number(threshold) }] })
      onCreated()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed to create quality gate') } finally { setSaving(false) }
  }

  return <Card title="New quality gate" className="mb-6"><form className="space-y-4" onSubmit={submit}><div className="grid gap-4 sm:grid-cols-2"><Field label="Name" htmlFor="gate-name"><Input id="gate-name" value={name} onChange={(e) => setName(e.target.value)} /></Field><Field label="Key" hint="Lowercase letters, numbers, and hyphens" htmlFor="gate-key"><Input id="gate-key" value={key} onChange={(e) => setKey(e.target.value)} /></Field></div><div className="grid gap-4 sm:grid-cols-3"><Field label="Metric" htmlFor="gate-metric"><Select id="gate-metric" value={metric} onValueChange={setMetric} ariaLabel="Metric" options={metrics.map((value) => ({ value, label: value }))} /></Field><Field label="Operator" htmlFor="gate-op"><Select id="gate-op" value={op} onValueChange={(value) => setOp(value as QualityGateCondition['op'])} ariaLabel="Operator" options={operators.map((value) => ({ value, label: value }))} /></Field><Field label="Threshold" htmlFor="gate-threshold"><Input id="gate-threshold" type="number" value={threshold} onChange={(e) => setThreshold(e.target.value)} /></Field></div>{error && <ErrorState message={error} />}<div className="flex justify-end"><Button variant="brand" type="submit" loading={saving}>Create gate</Button></div></form></Card>
}
