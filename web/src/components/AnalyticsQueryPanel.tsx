import type { AnalyticsQueryResult } from '../api';

/** Values and evidence come directly from the deterministic query result. */
export default function AnalyticsQueryPanel({ result }: { result: AnalyticsQueryResult }) {
  const utc = (s: string) => new Date(s).toISOString();
  const number = (n: number) => new Intl.NumberFormat(undefined, { maximumSignificantDigits: 8 }).format(n);
  return <details className="trace" open>
    <summary>Analytics evidence ({result.rows.length} metrics)</summary>
    <p className="small">{result.query.operation} · {result.query.window_hours} hours · {utc(result.start)} to {utc(result.end)}</p>
    <p className="muted small">{result.method}</p>
    {result.rows.map(row => <section key={row.id} aria-label={`Evidence for ${row.id}`}>
      <h3>{row.name || row.id} <span className="mono small">({row.id})</span></h3>
      <p>{row.value === null ? row.note : `${number(row.value)} ${row.unit}`} · {row.samples} samples</p>
      {row.warning ? <p className="error-note">{row.warning}</p> : null}
      {row.first && row.last ? <p className="muted small">First: {number(row.first.v)} {row.unit} at {utc(row.first.t)}. Last: {number(row.last.v)} {row.unit} at {utc(row.last.t)}.</p> : null}
      {row.buckets?.length ? <div style={{ overflowX: 'auto' }}><table className="table compact"><caption>Observed bucket means for {row.id}</caption><thead><tr><th>Start (UTC)</th><th>End (UTC)</th><th>Mean ({row.unit || 'value'})</th><th>Samples</th></tr></thead><tbody>{row.buckets.map(b => <tr key={b.start}><td>{utc(b.start)}</td><td>{utc(b.end)}</td><td>{number(b.value)}</td><td>{b.samples}</td></tr>)}</tbody></table></div> : null}
      {row.sha256 ? <details><summary>Sample fingerprint</summary><p className="mono small" style={{ overflowWrap: 'anywhere' }}>SHA-256 {row.sha256}</p></details> : null}
    </section>)}
  </details>;
}
