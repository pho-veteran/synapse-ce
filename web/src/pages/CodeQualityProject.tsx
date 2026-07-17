import { AlertTriangle, ArrowLeft, FolderGit2, Gauge, GitBranch, Play, ShieldAlert } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { CodeQualityReportView } from '../components/codequality/CodeQualityReportView'
import { ProjectActivityView } from '../components/codequality/ProjectActivityView'
import { Button, Card, EmptyState, ErrorState, Pill, Spinner } from '../components/ui'
import { api } from '../lib/api'
import type { Project, ProjectAnalysis, ProjectAnalysisCursor, ScanJob, ScanResult } from '../lib/types'

export function CodeQualityProject() {
  const { key = '' } = useParams()
  const location = useLocation()
  const activityPage = location.pathname.endsWith('/activity')
  const startError = (location.state as { analysisStartError?: string } | null)?.analysisStartError
  const [project, setProject] = useState<Project | null | undefined>(undefined)
  const [job, setJob] = useState<ScanJob | null>(null)
  const [analysis, setAnalysis] = useState<ScanResult | null>(null)
  const [analyses, setAnalyses] = useState<ProjectAnalysis[]>([])
  const [analysisCursor, setAnalysisCursor] = useState<ProjectAnalysisCursor | null>(null)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [coverage, setCoverage] = useState<File | null>(null)
  const [gates, setGates] = useState<{ key: string; name: string }[]>([])
  const poll = useRef<ReturnType<typeof setInterval> | null>(null)

  function stopPoll() {
    if (poll.current) clearInterval(poll.current)
    poll.current = null
  }

  async function refreshResult() {
    try {
      const [result, history] = await Promise.all([api.latestProjectAnalysis(key), api.projectAnalyses(key)])
      if (!result) throw new Error('Analysis completed but its result is unavailable')
      setAnalysis(result)
      setAnalyses(history.items)
      setAnalysisCursor(history.next)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load analysis result')
    }
  }

  async function loadOlder() {
    if (!analysisCursor || loadingOlder) return
    setLoadingOlder(true)
    try {
      const history = await api.projectAnalyses(key, analysisCursor)
      setAnalyses((current) => [...current, ...history.items])
      setAnalysisCursor(history.next)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load older analyses')
    } finally {
      setLoadingOlder(false)
    }
  }

  function startPoll() {
    stopPoll()
    poll.current = setInterval(async () => {
      try {
        const next = await api.projectAnalysisStatus(key)
        if (!next) throw new Error('Analysis status is unavailable')
        setJob(next)
        if (next.status === 'running') return
        stopPoll()
        if (next.status === 'succeeded') await refreshResult()
        else setError(next.error || 'Analysis failed')
      } catch (e) {
        stopPoll()
        setError(e instanceof Error ? e.message : 'Failed to refresh analysis status')
      }
    }, 1500)
  }

  useEffect(() => {
    let live = true
    setProject(undefined); setLoadError(null); setError(startError ?? null); setAnalysis(null); setAnalyses([]); setAnalysisCursor(null); setJob(null)
    Promise.all([api.getProject(key), api.projectAnalysisStatus(key), api.latestProjectAnalysis(key), api.projectAnalyses(key)])
      .then(([nextProject, nextJob, nextAnalysis, history]) => {
        if (!live) return
        setProject(nextProject); setJob(nextJob); setAnalysis(nextAnalysis); setAnalyses(history.items); setAnalysisCursor(history.next)
        if (nextJob?.status === 'running') startPoll()
        else if (nextJob?.status === 'failed') setError(nextJob.error || 'Analysis failed')
      })
      .catch((e) => { if (live) setLoadError(e instanceof Error ? e.message : 'Failed to load project') })
    return () => { live = false; stopPoll() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, startError])

  useEffect(() => {
    api.listQualityGates().then(setGates).catch(() => setGates([]))
  }, [])

  async function assignGate(gateId: string) {
    setError(null)
    try {
      setProject(await api.assignProjectGate(key, gateId))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to assign quality gate')
    }
  }

  async function run() {
    setError(null)
    try {
      const next = await api.startProjectAnalysis(key, coverage ?? undefined)
      setCoverage(null); setJob(next); startPoll()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to start analysis')
    }
  }

  if (loadError && project === undefined) return <div className="mx-auto max-w-6xl space-y-3"><ErrorState message={loadError} /><Link to="/code-quality" className="inline-flex items-center gap-1.5 text-sm text-branddim hover:underline"><ArrowLeft className="size-4" aria-hidden="true" /> All projects</Link></div>
  if (project === undefined) return <Spinner label="Loading project…" />
  if (!project) return null
  const running = job?.status === 'running'
  const status = running ? 'Analyzing' : job?.status === 'failed' ? 'Failed' : analysis ? 'Analyzed' : 'Not analyzed'

  return (
    <div className="mx-auto max-w-6xl animate-fade-in">
      <Link to="/code-quality" className="mb-4 inline-flex items-center gap-1.5 text-sm text-mutedfg transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50">
        <ArrowLeft className="size-4" aria-hidden="true" /> All projects
      </Link>
      <header className="bg-hero mb-6 rounded-xl border border-border p-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-branddim"><Gauge className="size-4" aria-hidden="true" />Code Quality project</div>
            <h1 className="truncate text-3xl font-bold tracking-tight">{project.name}</h1>
            <p className="mt-1.5 font-mono text-sm text-mutedfg">{project.key}</p>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Pill className="shrink-0 bg-elevated ring-1 ring-inset ring-border"><Gauge className="size-3" aria-hidden="true" /> {status}</Pill>
            <label className="cursor-pointer rounded-lg border border-border bg-bg px-3 py-2 text-sm text-mutedfg focus-within:outline-none focus-within:ring-2 focus-within:ring-brand/60" title={coverage?.name ?? 'Optional LCOV, Cobertura, or JaCoCo report'}>
              <span>{coverage ? coverage.name : 'Coverage report (optional)'}</span><input className="sr-only" type="file" accept=".info,.lcov,.xml,text/plain,application/xml,text/xml" disabled={running} onChange={(event) => setCoverage(event.target.files?.[0] ?? null)} />
            </label>
            <Button variant="brand" loading={running} disabled={running} onClick={run}><Play className="size-4" aria-hidden="true" />{analysis ? 'Run again' : 'Run analysis'}</Button>
          </div>
        </div>
        <dl className="mt-5 grid grid-cols-1 gap-4 border-t border-border pt-4 text-sm sm:grid-cols-2">
          <div className="min-w-0"><dt className="text-[10px] font-semibold uppercase tracking-[0.12em] text-subtlefg">Source</dt><dd className="mt-1.5 flex min-w-0 items-center gap-2 text-foreground"><FolderGit2 className="size-4 shrink-0 text-mutedfg" aria-hidden="true" /><span className="capitalize">{project.sourceBinding.kind}</span><span className="truncate font-mono text-xs leading-5" title={project.sourceBinding.value}>{project.sourceBinding.value}</span></dd></div>
          <div><dt className="text-[10px] font-semibold uppercase tracking-[0.12em] text-subtlefg">Quality gate</dt><dd className="mt-1.5"><select aria-label="Quality gate" value={project.gateId} disabled={running} onChange={(event) => assignGate(event.target.value)} className="h-8 rounded border border-border bg-bg px-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-brand/60"><option value="">Default / repository file</option>{gates.map((gate) => <option key={gate.key} value={gate.key}>{gate.name}</option>)}</select></dd></div>
          {project.sourceBinding.ref && <div><dt className="text-[10px] font-semibold uppercase tracking-[0.12em] text-subtlefg">Branch or tag</dt><dd className="mt-1.5 flex items-center gap-2 font-mono text-xs leading-5 text-foreground"><GitBranch className="size-4 shrink-0 text-mutedfg" aria-hidden="true" />{project.sourceBinding.ref}</dd></div>}
        </dl>
        {running && <div className="mt-5"><div className="mb-1.5 flex items-center justify-between text-xs"><span className="capitalize text-foreground">{job.stage || 'starting'}…</span><span className="font-mono tabular-nums text-mutedfg">{job.progress}%</span></div><div className="h-1.5 overflow-hidden rounded-full bg-elevated"><div className="h-full rounded-full bg-brand transition-[width] duration-500" style={{ width: `${Math.max(3, job.progress)}%` }} /></div></div>}
      </header>
      <nav className="mb-6 flex gap-4 border-b border-border" aria-label="Project views"><Link to={`/code-quality/projects/${encodeURIComponent(key)}`} aria-current={activityPage ? undefined : 'page'} className={`border-b-2 px-1 pb-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 ${activityPage ? 'border-transparent text-mutedfg' : 'border-brand text-foreground'}`}>Latest analysis</Link><Link to={`/code-quality/projects/${encodeURIComponent(key)}/activity`} aria-current={activityPage ? 'page' : undefined} className={`border-b-2 px-1 pb-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 ${activityPage ? 'border-brand text-foreground' : 'border-transparent text-mutedfg'}`}>Activity</Link></nav>
      {error && <div className="mb-6"><ErrorState message={error} /></div>}
      {activityPage ? <ProjectActivityView analyses={analyses} hasOlder={analysisCursor !== null} loadingOlder={loadingOlder} onLoadOlder={loadOlder} /> : <><>{analysis && <Card title="Security analysis" className="mb-6"><div className="grid grid-cols-2 gap-3 sm:grid-cols-4"><SecurityMetric label="Findings" value={analysis.findings.length} /><SecurityMetric label="Vulnerabilities" value={analysis.vulnerabilities.length} /><SecurityMetric label="Packages" value={analysis.components.length} /><SecurityMetric label="License issues" value={analysis.licenses.filter((l) => l.verdict !== 'allow').length} /></div>{analysis.completeness.warning && <p className="mt-4 flex items-start gap-2 text-xs text-medium"><AlertTriangle className="mt-0.5 size-4 shrink-0" />{analysis.completeness.warning}</p>}</Card>}</><CodeQualityReportView report={analysis?.codeQuality} empty={<Card title="Analysis"><EmptyState icon={running ? Gauge : ShieldAlert} title={running ? 'Analysis in progress' : 'No analyses yet'} hint={running ? 'Source acquisition and the unified quality + security pipeline are running.' : 'Run the project analysis to produce findings and metrics.'} /></Card>} /></>}
    </div>
  )
}

function SecurityMetric({ label, value }: { label: string; value: number }) {
  return <div className="rounded-lg border border-border bg-bg px-4 py-3"><div className="font-mono text-2xl font-semibold tabular-nums">{value.toLocaleString()}</div><div className="text-xs text-mutedfg">{label}</div></div>
}
