// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
import { useState } from 'react';
import { api, type Model } from '../api';
import { Card, Empty, ErrorNote, PageHero, Pill, TableScroll } from '../components/ui';

interface Finance {
  currency: string; complete: boolean; missing?: string[]; cost: number;
  monthly_net: number; net_12_months: number; payback_months?: number;
  cost_of_delay_30_days: number; assumptions: string[];
  benefits: { id: string; monthly: number; evidence: string }[];
}
interface Candidate {
  actions: string[]; status: string; reasons: string[]; score: number;
  finance: Finance; resources: Record<string, number>;
  simulation: { kpis: { kpi: string; before: number; after: number; low: number; high: number }[] };
}
interface Plan { candidates: Candidate[]; evaluated: number; truncated: boolean; model_version: string; note: string }
interface Snapshot { schema: number; captured_at: string; sha256: string; model: Model; config: unknown; unusable: Record<string, boolean> }
interface Review { base_version: string; candidate_version: string; before: Plan; after: Plan; changes: { from: string; to: string; before: number; after: number }[]; note: string }
interface Timeline { note: string; points: { elapsed: string; result: Candidate['simulation'] & { violations?: { text: string }[] } }[] }
const fmt = (v: number) => new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(v);
const post = <T,>(path: string, body: unknown) => api<T>(path, { method: 'POST', json: body });
function download(name: string, value: unknown) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json' }));
  const a = document.createElement('a'); a.href = url; a.download = name; a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export default function Planning() {
  const [maxActions, setMaxActions] = useState(3);
  const [budget, setBudget] = useState('');
  const [capacity, setCapacity] = useState('{}');
  const [plan, setPlan] = useState<Plan | null>(null);
  const [selected, setSelected] = useState<Candidate | null>(null);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [review, setReview] = useState<Review | null>(null);
  const [edge, setEdge] = useState('0');
  const [weight, setWeight] = useState('');
  const [timeline, setTimeline] = useState<Timeline | null>(null);
  const [horizon, setHorizon] = useState('24h');
  const [step, setStep] = useState('1h');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [source, setSource] = useState('');
  const options = () => {
    const parsed = JSON.parse(capacity);
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('Capacity must be a JSON object.');
    const amount = budget === '' ? undefined : Number(budget);
    if (amount !== undefined && (!Number.isFinite(amount) || amount < 0)) throw new Error('Enter a nonnegative budget.');
    return { max_actions: maxActions, max_candidates: 2000, budget: amount, capacity: parsed };
  };
  const run = async (fn: () => Promise<void>) => { setBusy(true); setError(''); try { await fn(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); } };
  const show = (p: Plan, origin: string) => { setPlan(p); setSelected(p.candidates[0] ?? null); setTimeline(null); setSource(origin); };
  return <>
    <PageHero eyebrow="Decision planning" title="Make the trade-offs visible" lede="Compare action portfolios against budget, capacity and change windows. Inspect declared financial assumptions, replay a captured baseline, and review model changes before approval." tint="purple" />
    <ErrorNote message={error} />
    <Card title="Planning constraints" aside={<Pill tone="info">Advisory · approval required</Pill>}>
      <div className="row">
        <label>Maximum actions <input aria-label="Maximum actions" type="number" min={1} max={6} value={maxActions} onChange={e => setMaxActions(Number(e.target.value))} /></label>
        <label>Implementation budget <input aria-label="Implementation budget" type="number" min={0} placeholder="Unbounded" value={budget} onChange={e => setBudget(e.target.value)} /></label>
        <label>Resource capacities (JSON) <input aria-label="Resource capacities" value={capacity} onChange={e => setCapacity(e.target.value)} placeholder='{"engineers": 4}' /></label>
      </div>
      <p className="muted">Amounts use the pack’s declared currency. A budget requires cost data for every selected action. Resource names must match planning.json.</p>
      <button className="btn" disabled={busy} onClick={() => run(async () => show(await post<Plan>('/api/v1/planning/portfolios', options()), 'Live baseline'))}>{busy ? 'Working…' : 'Compare portfolios'}</button>
    </Card>
    {plan && <Card title={`${source} · ${plan.evaluated} candidates`} aside={<Pill tone={plan.truncated ? 'warn' : 'ok'}>{plan.truncated ? 'Search limit reached' : 'Search complete'}</Pill>}>
      <p className="muted">{plan.note} Model {plan.model_version}. Showing the first 100 candidates.</p>
      <TableScroll><table className="table"><thead><tr><th>Actions</th><th>Status</th><th>Score</th><th>Cost</th><th>Monthly net</th></tr></thead><tbody>
        {plan.candidates.slice(0, 100).map(c => <tr key={c.actions.join('+')}>
          <td><button className="linklike" onClick={() => { setSelected(c); setTimeline(null); }}>{c.actions.join(' + ')}</button></td>
          <td>{c.status}</td><td>{fmt(c.score)}</td>
          <td>{c.finance.complete ? `${c.finance.currency} ${fmt(c.finance.cost)}` : 'Incomplete'}</td>
          <td>{c.finance.complete ? fmt(c.finance.monthly_net) : 'Unknown'}</td>
        </tr>)}
      </tbody></table></TableScroll>
      {!plan.candidates.length && <Empty>No candidate actions in this model.</Empty>}
      <button className="btn ghost" onClick={() => download('zyntra-portfolios.json', plan)}>Export comparison</button>
    </Card>}
    {selected && <Card title={selected.actions.join(' + ')}>
      {selected.reasons.map(reason => <p key={reason}>{reason}</p>)}
      {!selected.finance.complete ? <p>Financial data missing for {selected.finance.missing?.join(', ')}. Partial totals are not a complete estimate.</p> : <div className="grid-2">
        <p>12-month net estimate: <strong>{selected.finance.currency} {fmt(selected.finance.net_12_months)}</strong></p>
        <p>Payback: <strong>{selected.finance.payback_months === undefined ? 'Not reached' : `${fmt(selected.finance.payback_months)} months`}</strong></p>
        <p>30-day delay estimate: <strong>{fmt(selected.finance.cost_of_delay_30_days)}</strong></p>
      </div>}
      <ul>{selected.finance.benefits.map(b => <li key={b.id}>{b.id}: {fmt(b.monthly)}/month · {b.evidence}</li>)}</ul>
      <ul>{selected.finance.assumptions.map((a, i) => <li key={i}>{a}</li>)}</ul>
      <p className="muted">Financial estimates assume constant monthly benefit and cost. They do not include tax, discounting, ramp-up or causal attribution.</p>
      {source === 'Live baseline' && <>
        <div className="row"><label>Horizon <input aria-label="Timeline horizon" value={horizon} onChange={e => setHorizon(e.target.value)} /></label><label>Step <input aria-label="Timeline step" value={step} onChange={e => setStep(e.target.value)} /></label>
          <button className="btn ghost" disabled={busy} onClick={() => run(async () => setTimeline(await post<Timeline>('/api/v1/planning/timeline', { actions: selected.actions, horizon, step })))}>Simulate over time</button></div>
        {timeline && <><p className="muted">{timeline.note}</p><TableScroll><table className="table"><thead><tr><th>Elapsed</th><th>KPI values</th><th>Constraint breaches</th></tr></thead><tbody>{timeline.points.map(p => <tr key={p.elapsed}><td>{p.elapsed}</td><td>{p.result.kpis.map(k => `${k.kpi}: ${fmt(k.after)}`).join(' · ')}</td><td>{p.result.violations?.map(v => v.text).join('; ') || 'None'}</td></tr>)}</tbody></table></TableScroll></>}
      </>}
    </Card>}
    <div className="grid-2">
      <Card title="Capture and replay">
        <p>Export the model, KPI baseline, planning assumptions and freshness together. Replay never refreshes live sources. Checksums detect accidental edits; they do not authenticate who captured a file.</p>
        <div className="row"><button className="btn ghost" disabled={busy} onClick={() => run(async () => { const s = await api<Snapshot>('/api/v1/planning/snapshot'); setSnapshot(s); setReview(null); download('zyntra-snapshot.json', s); })}>Capture snapshot</button>
          <label>Import snapshot <input aria-label="Import snapshot" type="file" accept=".json,application/json" onChange={e => { const f = e.target.files?.[0]; if (f) void run(async () => { if (f.size > 900000) throw new Error('Snapshot exceeds 900 KB upload limit.'); const s = JSON.parse(await f.text()) as Snapshot; const p = await post<Plan>('/api/v1/planning/replay', { snapshot: s, options: options() }); setSnapshot(s); setReview(null); show(p, 'Captured baseline'); }); }} /></label></div>
        {snapshot && <><p>Captured {snapshot.captured_at}</p><button className="btn ghost" disabled={busy} onClick={() => run(async () => show(await post<Plan>('/api/v1/planning/replay', { snapshot, options: options() }), 'Captured baseline'))}>Replay snapshot</button></>}
      </Card>
      <Card title="Review an edge change">
        <p>Capture or import a baseline, then compare a proposed weight. Export the review for your model change process. This does not install a model.</p>
        {snapshot ? <>
          <label>Edge <select aria-label="Edge" value={edge} onChange={e => setEdge(e.target.value)}>{snapshot.model.edges?.map((e, i) => <option key={i} value={i}>{e.from} → {e.to} ({e.weight})</option>)}</select></label>
          <label>New weight <input aria-label="New weight" type="number" step="any" value={weight} onChange={e => setWeight(e.target.value)} /></label>
          <button className="btn ghost" disabled={busy || !snapshot.model.edges?.length || weight === ''} onClick={() => run(async () => { const e = snapshot.model.edges[Number(edge)]; const value = Number(weight); if (!e || !Number.isFinite(value)) throw new Error('Select an edge and enter a finite weight.'); setReview(await post<Review>('/api/v1/planning/review', { snapshot, changes: [{ from: e.from, to: e.to, weight: value }], options: options() })); })}>Compare model change</button>
          {review && <><p>{review.base_version} → {review.candidate_version}</p><p>Top plan before: {review.before.candidates[0]?.actions.join(' + ') || 'None'}</p><p>Top plan after: {review.after.candidates[0]?.actions.join(' + ') || 'None'}</p><p className="muted">{review.note}</p><button className="btn ghost" onClick={() => download('zyntra-model-review.json', review)}>Export model review</button></>}
        </> : <Empty>Capture a baseline to review a change.</Empty>}
      </Card>
    </div>
  </>;
}
