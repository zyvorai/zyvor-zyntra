import { useState } from 'react';
import { useApi } from '../hooks';
import { Card, Empty, ErrorNote, TableScroll } from './ui';
interface Page { events: {sequence: number; status: string; reason?: string; received_at: string; document: {at: string; from?: string}}[]; next: number; oldest: number; truncated: boolean }
export default function InputEvents({ channel }: { channel: string }) {
 const [after, setAfter] = useState(0);
 const events = useApi<Page>(`/api/v1/inputs/${encodeURIComponent(channel)}/events?after=${after}&limit=100`);
 return <Card title={`Retained events · ${channel}`}>
  <ErrorNote message={events.error} onRetry={events.reload} />
  <p className="muted">Accepted, late and quarantined events. Retention is bounded; exporting a page never reapplies it.</p>
  {events.data?.truncated && <p>Earlier events have expired. The oldest retained sequence is {events.data.oldest}.</p>}
  <TableScroll><table className="table compact"><thead><tr><th>Sequence</th><th>Status</th><th>Observed</th><th>Received</th><th>Reason</th></tr></thead><tbody>
   {(events.data?.events ?? []).map(e => <tr key={e.sequence}><td>{e.sequence}</td><td>{e.status}</td><td>{e.document.at}</td><td>{e.received_at}</td><td>{e.reason || '—'}</td></tr>)}
  </tbody></table></TableScroll>
  {events.data?.events.length === 0 && <Empty>No retained events after this cursor.</Empty>}
  <div className="row"><button className="btn ghost" onClick={() => setAfter(0)} disabled={after===0}>First retained events</button><button className="btn ghost" onClick={() => setAfter(events.data?.next ?? after)} disabled={!events.data?.events.length}>Next page</button></div>
 </Card>;
}
