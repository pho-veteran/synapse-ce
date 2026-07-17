import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

describe('Projects API', () => {
  let fetchSpy: any

  beforeEach(() => { fetchSpy = vi.spyOn(globalThis, 'fetch') })

  it('maps project fields and portfolio summaries', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [{ ID: 'p1', Name: 'Synapse', Key: 'synapse', SourceBinding: { Kind: 'git', Value: 'https://example.com/repo.git', Ref: 'main' }, DefaultProfileByLang: { go: 'default' }, GateID: 'gate', Audit: { CreatedAt: '2026-07-15T00:00:00Z' }, latest_analysis: { id: 'a1', gate: { passed: false, results: [] }, gate_info: { key: 'release', name: 'Release', source: 'managed' } }, latest_job: { id: 'j1', status: 'succeeded' } }] } as Response)
    const projects = await api.listProjects()
    expect(projects[0]).toMatchObject({ id: 'p1', key: 'synapse', gateId: 'gate', sourceBinding: { kind: 'git', ref: 'main' }, latestAnalysis: { id: 'a1', gateInfo: { name: 'Release', source: 'managed' } }, latestJob: { id: 'j1', status: 'succeeded' } })
    expect(fetchSpy).toHaveBeenCalledWith('/api/v1/projects', expect.any(Object))
  })

  it('marks absent engagement code-quality grades as unavailable', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ available: true, report: {} }) } as Response)
    const view = await api.codeQuality('e1')
    expect(view).toMatchObject({ available: true, report: { rating: { security: '?', reliability: '?', maintainability: '?' } } })
  })

  it('creates with the backend source and gate contract', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 201, json: async () => ({ ID: 'p1', Key: 'synapse', SourceBinding: { Kind: 'local', Value: '/repo' } }) } as Response)
    await api.createProject({ name: 'Synapse', key: 'synapse', gateId: 'release', sourceBinding: { kind: 'local', value: '/repo', ref: '' } })
    const init = fetchSpy.mock.calls[0][1] as RequestInit
    expect(fetchSpy.mock.calls[0][0]).toBe('/api/v1/projects')
    expect(JSON.parse(String(init.body))).toEqual({ name: 'Synapse', key: 'synapse', gate_id: 'release', source_binding: { Kind: 'local', Value: '/repo', Ref: '' } })
  })

  it('manages quality gates', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [{ key: 'synapse-way', name: 'Synapse way', built_in: true, conditions: [] }] } as Response)
    expect(await api.listQualityGates()).toMatchObject([{ key: 'synapse-way', builtIn: true }])

    fetchSpy.mockResolvedValueOnce({ ok: true, status: 201, json: async () => ({ key: 'release', name: 'Release', conditions: [] }) } as Response)
    await api.createQualityGate({ key: 'release', name: 'Release', conditions: [] })
    expect(fetchSpy).toHaveBeenLastCalledWith('/api/v1/quality-gates', expect.objectContaining({ method: 'POST' }))
  })

  it('encodes project keys', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ Key: 'a b' }) } as Response)
    await api.getProject('a b')
    expect(fetchSpy).toHaveBeenCalledWith('/api/v1/projects/a%20b', expect.any(Object))
  })

  it('keeps an unavailable first delta null', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ items: [{ id: 'a1', delta: null, gate: { passed: true, results: [] } }] }) } as Response)
    const page = await api.projectAnalyses('project')
    expect(page.items[0].delta).toBeNull()
  })

  it('uploads coverage as multipart but keeps no-file requests unchanged', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 202, json: async () => ({ id: 'job-1' }) } as Response)
    await api.startProjectAnalysis('project')
    expect(fetchSpy.mock.calls[0][1].headers).toMatchObject({ 'content-type': 'application/json' })

    fetchSpy.mockResolvedValueOnce({ ok: true, status: 202, json: async () => ({ id: 'job-2' }) } as Response)
    await api.startProjectAnalysis('project', new File(['SF:a.go\nDA:1,1\n'], 'coverage.info'))
    const init = fetchSpy.mock.calls[1][1] as RequestInit
    expect(init.body).toBeInstanceOf(FormData)
    expect(init.headers).not.toHaveProperty('content-type')
  })

  it('maps and sends project analysis cursors', async () => {
    fetchSpy.mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({
      items: [{ id: 'a1', created_at: '2026-07-17T00:00:00Z', gate: { passed: true, results: [] }, coverage: null }],
      next: { before_created_at: '2026-07-16T00:00:00Z', before_id: 'a0' },
    }) } as Response)
    const page = await api.projectAnalyses('a b', { beforeCreatedAt: '2026-07-18T00:00:00Z', beforeId: 'a2' })
    expect(page.items[0]).toMatchObject({ id: 'a1', coverage: null, rating: { security: '?', reliability: '?', maintainability: '?' }, newCode: { rating: { security: '?', reliability: '?', maintainability: null } } })
    expect(page.next).toEqual({ beforeCreatedAt: '2026-07-16T00:00:00Z', beforeId: 'a0' })
    expect(fetchSpy).toHaveBeenCalledWith('/api/v1/projects/a%20b/analyses?limit=25&before_created_at=2026-07-18T00%3A00%3A00Z&before_id=a2', expect.any(Object))
  })
})
