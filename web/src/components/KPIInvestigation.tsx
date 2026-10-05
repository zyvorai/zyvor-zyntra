import { useState, type FormEvent } from 'react';
import { api, type AnalyticsMetric, type InvestigationReport } from '../api';
import { useApi } from '../hooks';
import { Card, Empty, ErrorNote, Pill } from './ui';

export default function KPIInvestigation() {
  const catalog = useApi<{ metrics: AnalyticsMetric[] }>('/api/v1/analytics/catalog');
  const [metric, setMetric] = useState('');
  const [windowHours, setWindowHours] = useState(168);
  const [recentHours, setRecentHours] = useState(6);
  const [maxLag, setMaxLag] = useState(6);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [report, setReport] = useState<InvestigationReport | null>(null);
  const selected = metric || catalog.data?.metrics[0]?.id || '';
  const run = async (e: FormEvent) => {
    e.preventDefault();
    if (busy || !selected) return;
    setReport(null); setError(''); setBusy(true);
    try {
      setReport(await api<InvestigationReport>('/api/v1/analytics/investigate', { method: 'POST', json: {
        metric: selected, window_hours: windowHours, recent_hours: recentHours, max_lag_hours: maxLag,
      } }));
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  };
  return <Card title="Investigate a KPI">
    <p className="muted small">Check for sustained changes and explore metrics whose hourly changes preceded or moved with the target. Every calculation uses recorded observations.</p>
    <ErrorNote message={catalog.error || error} />
    {catalog.data?.metrics.length === 0 ? <Empty>No permitted metrics in this pack.</Empty> : null}
    <form onSubmit={run}>
      <fieldset disabled={busy || !catalog.data?.metrics.length}>
        <legend className="small">Investigation settings</legend>
        <div className="grid-2">
          <label>Target metric<select aria-label="Investigation target" value={selected} onChange={e => setMetric(e.target.value)}>
            {catalog.data?.metrics.map(m => <option key={m.id} value={m.id}>{m.name || m.id} ({m.id})</option>)}
          </select></label>
          <label>Window (hours)<input aria-label="Investigation window hours" type="number" min={24} max={720} step={1} required value={windowHours} onChange={e => setWindowHours(Number(e.target.value))} /></label>
          <label>Recent period (hours)<input aria-label="Investigation recent hours" type="number" min={3} max={Math.min(24, windowHours - 12)} step={1} required value={recentHours} onChange={e => setRecentHours(Number(e.target.value))} /></label>
          <label>Maximum candidate lead (hours)<input aria-label="Investigation maximum lag hours" type="number" min={0} max={12} step={1} required value={maxLag} onChange={e => setMaxLag(Number(e.target.value))} /></label>
        </div>
        <button className="btn-secondary" type="submit">{busy ? 'Investigating…' : 'Investigate'}</button>
      </fieldset>
    </form>
    {report ? <InvestigationEvidence report={report} /> : null}
  </Card>;
}

export function InvestigationEvidence({ report }: { report: InvestigationReport }) {
  const shift = report.shift;
  const associations = report.comparisons.filter(c => c.status === 'associated').slice(0, 5);
  const unit = report.target.unit;
  const measure = (n: number) => new Intl.NumberFormat(undefined, { maximumSignificantDigits: 6, notation: Math.abs(n) >= 1e9 || (n !== 0 && Math.abs(n) < 1e-6) ? 'scientific' : 'standard' }).format(n);
  const utc = (s: string) => new Date(s).toISOString();
  const download = () => {
    const url = URL.createObjectURL(new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' }));
    const a = document.createElement('a'); a.href = url; a.download = 'zyntra-kpi-investigation.json'; document.body.appendChild(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  };
  return <div className="info-note" aria-label={`Investigation for ${report.target.id}`}>
    <h3>{report.target.name || report.target.id} <span className="mono small">({report.target.id})</span></h3>
    <Pill tone={shift.status === 'shift' ? 'warn' : 'neutral'}>{shift.status === 'shift' ? `Sustained shift ${shift.direction}` : shift.status}</Pill>
    <p className="muted small">{utc(report.start)} to {utc(report.end)} (end exclusive; completed hours only). Recent period: {report.query.recent_hours}h. Candidate leads tested: 0–{report.query.max_lag_hours}h.</p>
    {report.target.warning ? <p className="error-note">{report.target.warning}</p> : null}
    <p>{shift.reason}</p>
    {shift.delta !== undefined ? <p>Baseline median: {measure(shift.baseline_median!)} {unit} over {shift.baseline_hours} consecutive hours. Recent median: {measure(shift.recent_median!)} {unit} over {shift.recent_hours} hours. Change: {measure(shift.delta)} {unit}. Threshold: {measure(shift.threshold!)} {unit}; {shift.beyond_threshold} hours exceeded it; {shift.required_hours} are required for a sustained shift.</p> : null}
    <p className="muted small">{report.caveat}</p>
    {report.candidates_truncated ? <p className="error-note">Candidate search capped at 64 of {report.candidate_total} permitted metrics, selected by metric ID.</p> : null}
    {associations.length ? <div style={{ overflowX: 'auto' }}><table className="table compact">
      <caption>Strongest exploratory associations for {report.target.id} (up to 5)</caption>
      <thead><tr><th>Candidate</th><th>Change correlation</th><th>Candidate lead</th><th>Aligned pairs</th><th>Eligible lags</th></tr></thead>
      <tbody>{associations.map(c => <tr key={c.id}><td>{c.name || c.id}{c.warning ? <p className="small">{c.warning}</p> : null}</td><td>{c.correlation!.toFixed(3)}</td><td>{c.lag_hours === 0 ? 'Same hour' : `${c.lag_hours}h earlier`}</td><td>{c.pairs}</td><td>{c.tested_lags}</td></tr>)}</tbody>
    </table></div> : <p>No eligible candidate reached the exploratory association threshold.</p>}
    <details className="trace"><summary>Method and comparison diagnostics ({report.comparisons.length})</summary>
      <p className="small">{report.method}</p>
      {report.comparisons.map(c => <div key={c.id}>
        <strong>{c.id}</strong> · {c.status} · {c.pairs} pairs · {c.tested_lags} eligible lags
        {c.correlation !== undefined ? <span> · r {c.correlation.toFixed(3)} · lead {c.lag_hours}h</span> : null}
        <p className="muted small">{c.reason}</p>
        {c.warning ? <p className="error-note">{c.warning}</p> : null}
        {c.start && c.end ? <p className="small">Target change times: {utc(c.start)} to {utc(c.end)}</p> : null}
        {c.sha256 ? <p className="mono small" style={{ overflowWrap: 'anywhere' }}>Aligned-pair SHA-256 {c.sha256}</p> : null}
        {c.evidence?.length ? <details><summary>Aligned change evidence for {c.id} ({c.evidence.length} pairs)</summary><p className="small">Latest 12 pairs shown; all pairs for up to five scored comparisons are included in the JSON export. Candidate change time is target time minus the reported lead.</p><div style={{ overflowX: 'auto' }}><table className="table compact"><caption>Aligned changes for {c.id}</caption><thead><tr><th>Target change time (UTC)</th><th>Candidate change ({c.unit || 'value'})</th><th>Target change ({unit || 'value'})</th></tr></thead><tbody>{c.evidence.slice(-12).map(p => <tr key={p.at}><td>{utc(p.at)}</td><td>{measure(p.candidate_change)}</td><td>{measure(p.target_change)}</td></tr>)}</tbody></table></div></details> : null}
      </div>)}
    </details>
    <details className="trace"><summary>Target hourly evidence ({report.hours.length} hours)</summary>
      {report.sha256 ? <p className="mono small" style={{ overflowWrap: 'anywhere' }}>Hourly SHA-256 {report.sha256}</p> : null}
      <p className="muted small">Most recent 24 observed hours shown; the JSON export includes all observed hours in the window.</p>
      {report.hours.length ? <div style={{ overflowX: 'auto' }}><table className="table compact"><caption>Recent observed hours for {report.target.id}</caption>
        <thead><tr><th>Hour start (UTC)</th><th>Mean ({unit || 'value'})</th><th>Samples</th></tr></thead>
        <tbody>{report.hours.slice(-24).map(h => <tr key={h.start}><td>{utc(h.start)}</td><td>{measure(h.mean)}</td><td>{h.samples}</td></tr>)}</tbody>
      </table></div> : <p>No usable completed hours.</p>}
    </details>
    <button className="btn-secondary" onClick={download}>Download investigation JSON</button>
  </div>;
}
