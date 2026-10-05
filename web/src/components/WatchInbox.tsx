import { useState, type FormEvent } from 'react';
import { api, type AnalyticsMetric, type WatchRule, type WatchView } from '../api';
import { useApi } from '../hooks';
import { useWho } from '../session';
import { Card, Empty, ErrorNote, Pill } from './ui';
import IncidentInvestigationPanel from './IncidentInvestigation';

const blank = {id:'',name:'',metric:'',operator:'above' as 'above'|'below',threshold:'0',for_seconds:300,clear_seconds:60,max_gap_seconds:120,enabled:true,version:0};
export default function WatchInbox() {
 const who=useWho(); const roles=who?.identity.roles ?? (who ? [who.identity.role] : []);
 const admin=!!who && !who.identity.tenant && roles.includes('admin');
 const canAck=roles.some(r=>['admin','approver','proposer'].includes(r));
 const list=useApi<WatchView>('/api/v1/watches',15000);
 const catalog=useApi<{metrics:AnalyticsMetric[]}>('/api/v1/analytics/catalog');
 const [draft,setDraft]=useState(blank);const [editing,setEditing]=useState(false);
 const [busy,setBusy]=useState(false);const [note,setNote]=useState('');const [remove,setRemove]=useState<WatchRule|null>(null);
 const [ack,setAck]=useState<string|null>(null);const [ackNote,setAckNote]=useState('');
 const save=async(e:FormEvent)=>{
  e.preventDefault();if(busy)return;setBusy(true);setNote('');
  try {
   const {id,version,...values}=draft;
   await api(`/api/v1/watches/${encodeURIComponent(id)}`,{method:'PUT',json:{...values,threshold:Number(draft.threshold),expected_version:version}});
   setEditing(false);await list.reload();setNote('Watch rule saved.');
  }catch(e){setNote((e as Error).message)}finally{setBusy(false)}
 };
 const acknowledge=async(e:FormEvent)=>{
  e.preventDefault();const incident=list.data?.incidents.find(i=>i.id===ack);if(!incident||busy)return;setBusy(true);setNote('');
  try {await api(`/api/v1/watch-incidents/${encodeURIComponent(incident.id)}/acknowledge`,{method:'POST',json:{expected_version:incident.version,note:ackNote}});setAck(null);setAckNote('');await list.reload();setNote('Incident acknowledged.');}
  catch(e){setNote((e as Error).message)}finally{setBusy(false)}
 };
 const deleteRule=async()=>{
  if(!remove||busy)return;setBusy(true);setNote('');
  try{await api(`/api/v1/watches/${encodeURIComponent(remove.id)}`,{method:'DELETE',json:{expected_version:remove.version}});setRemove(null);await list.reload();setNote('Rule deleted; retained incident history remains.');}
  catch(e){setNote((e as Error).message)}finally{setBusy(false)}
 };
 const measure=(n:number)=>new Intl.NumberFormat(undefined,{maximumSignificantDigits:6}).format(n);
 const active=list.data?.incidents.filter(i=>i.status!=='resolved') ?? [];
 return <Card title="KPI watch inbox" aside={<Pill tone={active.length?'warn':'neutral'}>{active.length} active</Pill>}>
  <p className="muted small">Sustained threshold watches evaluate fresh live observations. Missing or failed data cannot establish breach or recovery duration. Acknowledgement records ownership; it does not close an incident or run an action.</p>
  <ErrorNote message={list.error || list.data?.persistence_error || ''} />
  {note ? <p role="status" className="small">{note}</p> : null}
  {list.data?.incidents.length===0 ? <Empty>No retained watch incidents.</Empty> : null}
  {list.data?.incidents.map(i=><details key={i.id} className="trace" open={i.status!=='resolved'}><summary>{i.rule.name} · {i.status} · {i.id}</summary>
   <p>{i.rule.metric_name || i.rule.metric} ({i.rule.metric}) {i.rule.operator} {measure(i.rule.threshold)} {i.rule.unit} for {i.rule.for_seconds}s · rule v{i.rule.version}</p>
   {list.data?.runtime[i.rule.id]?.status==='unavailable' && list.data?.runtime[i.rule.id]?.active_incident===i.id ? <p className="error-note">Current observations are unavailable; this incident has not been cleared.</p> : null}
   <p className="small">Opened {new Date(i.opened_at).toLocaleString()}. Last recorded evidence: {measure(i.value)} {i.rule.unit} at {new Date(i.observed_at).toLocaleString()}.</p>
   {i.acknowledged_by ? <p className="small">Acknowledged by {i.acknowledged_by}: {i.acknowledgement_note}</p> : null}
   {i.resolved_at ? <p className="small">Resolved {new Date(i.resolved_at).toLocaleString()}: {i.resolve_reason}</p> : null}
   <IncidentInvestigationPanel id={i.id} />
   {canAck && i.status==='open' ? <button className="btn-secondary" disabled={busy} onClick={()=>{setAck(i.id);setAckNote('');setNote('');}}>Acknowledge {i.id}</button> : null}
   {ack===i.id && i.status==='open' ? <form onSubmit={acknowledge}><label>Acknowledgement note<textarea aria-label="Acknowledgement note" required maxLength={500} value={ackNote} onChange={e=>setAckNote(e.target.value)} /></label><button className="btn-secondary" disabled={busy||!ackNote.trim()}>Confirm acknowledgement</button><button type="button" className="btn-secondary" onClick={()=>setAck(null)}>Cancel acknowledgement</button></form> : null}
  </details>)}
  <details className="trace"><summary>Watch rules ({list.data?.rules.length ?? 0})</summary>
   {list.data?.rules.map(r=><div className="info-note" key={r.id}><strong>{r.name}</strong> · {list.data?.runtime[r.id]?.status || 'waiting'}
    <p className="small">{r.metric} {r.operator} {measure(r.threshold)} {r.unit}; breach {r.for_seconds}s, recovery {r.clear_seconds}s, maximum observation gap {r.max_gap_seconds}s. {r.enabled?'Enabled':'Disabled'} · v{r.version}</p>
    {admin ? <div className="pills"><button className="btn-secondary" disabled={busy} onClick={()=>{setDraft({id:r.id,name:r.name,metric:r.metric,operator:r.operator,threshold:String(r.threshold),for_seconds:r.for_seconds,clear_seconds:r.clear_seconds,max_gap_seconds:r.max_gap_seconds,enabled:r.enabled,version:r.version});setEditing(true);setRemove(null);}}>Edit {r.id}</button><button className="btn-secondary" disabled={busy} onClick={()=>setRemove(r)}>Delete {r.id}</button></div> : null}
   </div>)}
  </details>
  {admin ? <button className="btn-secondary" disabled={busy} onClick={()=>{setDraft({...blank,metric:catalog.data?.metrics[0]?.id || ''});setEditing(true);setRemove(null);setNote('');}}>Add watch rule</button> : null}
  {remove && admin ? <div className="info-note"><p>Delete {remove.name}? Its active incident closes as “rule deleted”; retained incident history stays visible.</p><button className="btn-secondary" disabled={busy} onClick={deleteRule}>Confirm rule deletion</button><button className="btn-secondary" onClick={()=>setRemove(null)}>Cancel rule deletion</button></div> : null}
  {editing && admin ? <form onSubmit={save}>
   <p className="small">Saving changes closes the current incident as “rule changed” and resets duration timers. Metric tenant scope cannot change.</p>
   <ErrorNote message={catalog.error} />
   <fieldset disabled={busy}><legend>Watch rule settings</legend><div className="grid-2">
    <label>Rule ID<input aria-label="Watch rule ID" required pattern="[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}" readOnly={draft.version>0} value={draft.id} onChange={e=>setDraft({...draft,id:e.target.value})}/></label>
    <label>Name<input aria-label="Watch rule name" required maxLength={120} value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})}/></label>
    <label>Metric<select aria-label="Watch metric" required value={draft.metric} onChange={e=>setDraft({...draft,metric:e.target.value})}>{catalog.data?.metrics.map(m=><option key={m.id} value={m.id}>{m.name || m.id} ({m.id})</option>)}</select></label>
    <label>Condition<select aria-label="Watch condition" value={draft.operator} onChange={e=>setDraft({...draft,operator:e.target.value as 'above'|'below'})}><option value="above">Above</option><option value="below">Below</option></select></label>
    <label>Threshold<input aria-label="Watch threshold" type="number" step="any" required value={draft.threshold} onChange={e=>setDraft({...draft,threshold:e.target.value})}/></label>
    <label>Breach duration (seconds)<input aria-label="Watch breach seconds" type="number" min={0} max={86400} step={1} required value={draft.for_seconds} onChange={e=>setDraft({...draft,for_seconds:Number(e.target.value)})}/></label>
    <label>Recovery duration (seconds)<input aria-label="Watch recovery seconds" type="number" min={0} max={86400} step={1} required value={draft.clear_seconds} onChange={e=>setDraft({...draft,clear_seconds:Number(e.target.value)})}/></label>
    <label>Maximum observation gap (seconds)<input aria-label="Watch maximum gap seconds" type="number" min={1} max={3600} step={1} required value={draft.max_gap_seconds} onChange={e=>setDraft({...draft,max_gap_seconds:Number(e.target.value)})}/></label>
    <label><input aria-label="Watch enabled" type="checkbox" checked={draft.enabled} onChange={e=>setDraft({...draft,enabled:e.target.checked})}/> Enabled</label>
   </div><button className="btn-secondary" type="submit">Save watch rule</button><button type="button" className="btn-secondary" onClick={()=>setEditing(false)}>Cancel editing</button></fieldset>
  </form> : null}
  <details className="trace"><summary>Retained watch events ({list.data?.events.length ?? 0})</summary><ul>{list.data?.events.map(e=><li key={e.sequence}>{new Date(e.at).toLocaleString()} · {e.rule} · {e.kind} · {e.by}{e.note ? ` · ${e.note}` : ''}</li>)}</ul></details>
 </Card>;
}
