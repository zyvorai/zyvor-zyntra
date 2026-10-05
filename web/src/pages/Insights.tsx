import { duration, fmt, pct, type AIStatus, type AnalyticsReport, type Anomaly, type CalibrationReport, type Forecast } from '../api';
import { useApi } from '../hooks';
import type { Page } from '../nav';
import { Card, Empty, ErrorNote, PageHero, Pill } from '../components/ui';

import KPIInvestigation from '../components/KPIInvestigation';
import WatchInbox from '../components/WatchInbox';

const fcTone = { breach: 'bad', 'at-risk': 'warn', improving: 'ok', stable: 'neutral' } as const;

/** How well past predictions matched what happened, and suggested weight fixes. */
function Calibration() {
  const { data } = useApi<CalibrationReport>('/api/v1/ai/calibration', 60000);
  if (!data) return null;
  const actionFixes = data.action_suggestions ?? [];
  return (
    <Card title="Model calibration" aside={<Pill tone={data.suggestions.length || actionFixes.length ? 'warn' : 'neutral'}>{data.decisions} decision(s)</Pill>}>
      <p className="muted small">
        Backtests the model's edge weights and action effects against changes that actually ran. Suggestions are never applied automatically; review them and edit the pack.
      </p>
      {data.note ? <Empty>{data.note}</Empty> : null}
      {data.kpis.length ? (
        <table className="table compact">
          <thead>
            <tr>
              <th>KPI</th>
              <th className="num">Decisions</th>
              <th className="num">Mean error</th>
              <th className="num">Hit rate</th>
            </tr>
          </thead>
          <tbody>
            {data.kpis.map((k) => (
              <tr key={k.kpi}>
                <td className="mono">{k.kpi}</td>
                <td className="num">{k.n}</td>
                <td className="num">{pct(k.mean_abs_error)}</td>
                <td className="num">{pct(k.hit_rate)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      {data.suggestions.map((s) => (
        <div key={s.kpi} className="info-note">
          <p>{s.why}</p>
          <pre className="code">{s.yaml}</pre>
        </div>
      ))}
      {actionFixes.map((s) => (
        <div key={`${s.action}:${s.kpi}`} className="info-note">
          <p>{s.why}</p>
          <pre className="code">{s.yaml}</pre>
        </div>
      ))}
      {data.notes?.map((n) => (
        <p key={n} className="muted small">
          No change — {n}
        </p>
      ))}
    </Card>
  );
}

export default function Insights({ setPage }: { setPage: (p: Page) => void }) {
  const ins = useApi<{ anomalies: Anomaly[]; forecasts: Forecast[]; analytics?: AnalyticsReport }>('/api/v1/ai/insights', 15000);
  const st = useApi<AIStatus>('/api/v1/ai/status');
  const an = ins.data?.anomalies ?? [];
  const fc = (ins.data?.forecasts ?? []).filter((f) => f.status !== 'stable');

  return (
    <>
      <PageHero
        eyebrow="Intelligence"
        title="Insights"
        tint="purple"
        lede="Explore KPI anomalies, seasonal projections and historical forecast accuracy. Every number is calculated from recorded observations."
        actions={
          <button className="btn-diag" onClick={() => setPage('ask')}>
            Ask Zyntra
          </button>
        }
      />
      <ErrorNote message={ins.error} />
      {st.data ? (
        <p className="muted small">
          AI mode <strong>{st.data.mode}</strong>
          {st.data.provider ? ` · ${st.data.provider}` : ''}
          {st.data.model ? ` · ${st.data.model}` : ''} · {st.data.mutations}
        </p>
      ) : null}
      <div className="grid-2">
        <Card title="Anomalies" aside={<Pill tone={an.length ? 'warn' : 'ok'}>{an.length}</Pill>}>
          {an.length === 0 ? (
            <Empty>No KPI is more than 3σ from its baseline.</Empty>
          ) : (
            <ul className="list">
              {an.map((a) => (
                <li key={a.kpi}>
                  <div className="list-main">
                    <strong>{a.name}</strong>
                    <span className="muted">{a.text}</span>
                  </div>
                  <Pill tone={a.severity === 'critical' ? 'bad' : 'warn'}>z {a.z.toFixed(1)}</Pill>
                </li>
              ))}
            </ul>
          )}
        </Card>
        <Card title="Forecasts" aside={<Pill tone={fc.some((f) => f.status === 'at-risk') ? 'warn' : 'neutral'}>{fc.length}</Pill>}>
          {fc.length === 0 ? (
            <Empty>No KPI is trending toward or away from its target yet. Forecasts need a few minutes of history.</Empty>
          ) : (
            <ul className="list">
              {fc.map((f) => (
                <li key={f.kpi}>
                  <div className="list-main">
                    <strong>{f.name}</strong>
                    <span className="muted">
                      {fmt(f.value)} → target {fmt(f.target)} · {f.slope_per_hour >= 0 ? '+' : ''}
                      {fmt(f.slope_per_hour)}/h
                      {f.eta_seconds !== undefined ? ` · ${f.status === 'at-risk' ? 'breach' : 'recovery'} in ${duration(f.eta_seconds)}` : ''}
                    </span>
                  </div>
                  <Pill tone={fcTone[f.status]}>{f.status}</Pill>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
      {ins.data?.analytics ? <Analytics data={ins.data.analytics} /> : null}
      <WatchInbox />
      <KPIInvestigation />
      <Calibration />
    </>
  );
}

function Analytics({ data }: { data: AnalyticsReport }) {
 return <>
  <ErrorNote message={data.persistence_error ?? ''} />
  <Card title="Robust anomalies" aside={<Pill tone={data.anomalies.length ? 'warn' : 'neutral'}>{data.anomalies.length}</Pill>}>
   <p className="muted small">Completed-hour means against a median/MAD baseline, using the same daily hour when enough history exists.</p>
   {data.anomalies.length === 0 ? <Empty>No robust anomaly detected, or insufficient baseline history.</Empty> : <ul className="list">
    {data.anomalies.map(a => <li key={a.kpi}><div className="list-main"><strong>{a.name}</strong><span className="muted">{a.text}</span></div><Pill tone={a.severity === 'critical' ? 'bad' : 'warn'}>{a.method}</Pill></li>)}
   </ul>}
  </Card>
  <Card title="Seasonal analytics" aside={<Pill tone="neutral">{data.persistent ? 'Persistent history' : 'Memory history'}</Pill>}>
   <p className="muted small">Models compete against a last-value baseline on 24 rolling one-hour predictions. Lower MAE wins; daily and weekly models repeat observed seasonal values.</p>
   <p className="muted small">{data.interval_note}</p>
   {data.kpis.map(k => <div key={k.kpi} className="info-note">
    <strong>{k.name || k.kpi}</strong> · {k.observations} observations · {k.hourly_samples} consecutive hourly samples
    {k.status !== 'ready' ? <p className="muted small">{k.reason}</p> : <>
     <p>Selected model: <strong>{k.selected}</strong></p>
     <table className="table compact"><caption>Forecast accuracy for {k.name || k.kpi}</caption><thead><tr><th>Model</th><th className="num">MAE</th><th className="num">RMSE</th><th className="num">Origins</th></tr></thead><tbody>
      {k.backtests.map(b => <tr key={b.method}><td>{b.method}</td><td className="num">{fmt(b.mae)}</td><td className="num">{fmt(b.rmse)}</td><td className="num">{b.samples}</td></tr>)}
     </tbody></table>
     <table className="table compact"><caption>Projections for {k.name || k.kpi}</caption><thead><tr><th>Horizon</th><th className="num">Value ({k.unit})</th><th className="num">Empirical band</th><th>Target</th></tr></thead><tbody>
      {k.projections.map(p => <tr key={p.hours}><td>{p.hours}h</td><td className="num">{fmt(p.value)}</td><td className="num">{fmt(p.low)} – {fmt(p.high)}</td><td><Pill tone={p.breach ? 'warn' : 'neutral'}>{p.breach ? 'Projected miss' : 'No projected miss'}</Pill></td></tr>)}
     </tbody></table>
    </>}
   </div>)}
  </Card>
 </>;
}
