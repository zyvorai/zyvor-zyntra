import { useState } from 'react';
import { api, type IncidentInvestigation } from '../api';
import { InvestigationEvidence } from './KPIInvestigation';
import { ErrorNote } from './ui';

export default function IncidentInvestigationPanel({ id }: { id: string }) {
 const [result, setResult] = useState<IncidentInvestigation | null>(null);
 const [busy, setBusy] = useState(false);
 const [error, setError] = useState('');
 const [visible, setVisible] = useState(false);
 const run = async () => {
  if (busy) return;
  setBusy(true); setError(''); setResult(null); setVisible(true);
  try { setResult(await api<IncidentInvestigation>(`/api/v1/watch-incidents/${encodeURIComponent(id)}/investigation`)); }
  catch (e) { setError((e as Error).message); }
  finally { setBusy(false); }
 };
 return <div>
  <button className="btn-secondary" disabled={busy} onClick={run}>{busy ? `Investigating ${id}…` : `Investigate ${id}`}</button>
  {visible ? <button className="btn-secondary" onClick={() => setVisible(false)}>Hide investigation {id}</button> : null}
  {visible ? <div aria-label={`Incident investigation ${id}`}>
   <ErrorNote message={error} />
   {result ? <>
    <h3>Historical context for {result.incident.id}</h3>
    <p className="small">Opened {new Date(result.incident.opened_at).toISOString()} · generated {new Date(result.generated_at).toISOString()} · rule v{result.incident.rule.version} · incident {result.incident.status}</p>
    <p className="muted small">{result.context}</p>
    <InvestigationEvidence report={result.report} exportValue={result} filename={`zyntra-incident-${id}.json`} />
   </> : null}
  </div> : null}
 </div>;
}
