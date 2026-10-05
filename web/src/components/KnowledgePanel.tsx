import { useState } from 'react';
import { api, type KnowledgeDocument } from '../api';
import { useApi } from '../hooks';
import { useWho } from '../session';
import { Card, Empty, ErrorNote, Pill } from './ui';

const emptyDocument: KnowledgeDocument = {id: '', title: '', source: '', text: '', visibility: 'provider', roles: [], version: 0, sha256: '', updated_at: '', updated_by: ''};

export default function KnowledgePanel() {
 const who = useWho();
 const admin = !!who && !who.identity.tenant && (who.identity.roles ?? [who.identity.role]).includes('admin');
 const list = useApi<{documents: KnowledgeDocument[]}>('/api/v1/knowledge/documents');
 const [selected, setSelected] = useState<KnowledgeDocument | null>(null);
 const [draft, setDraft] = useState<KnowledgeDocument>(emptyDocument);
 const [editing, setEditing] = useState(false);
 const [busy, setBusy] = useState(false);
 const [note, setNote] = useState('');
 const [removing, setRemoving] = useState(false);
 const open = async (id: string) => {
  setNote(''); setRemoving(false); setEditing(false);
  try {setSelected(await api<KnowledgeDocument>(`/api/v1/knowledge/documents/${encodeURIComponent(id)}`));}
  catch(e) {setNote((e as Error).message);}
 };
 const save = async () => {
  setBusy(true); setNote('');
  try {
   const d = await api<KnowledgeDocument>(`/api/v1/knowledge/documents/${encodeURIComponent(draft.id)}`, {method:'PUT', json:{document:draft, expected_version:draft.version}});
   setSelected(d); setEditing(false); await list.reload(); setNote('Document saved.');
  } catch(e) {setNote((e as Error).message);} finally {setBusy(false);}
 };
 const remove = async () => {
  if (!selected) return;
  setBusy(true);setNote('');
  try {
   await api(`/api/v1/knowledge/documents/${encodeURIComponent(selected.id)}`, {method:'DELETE',json:{expected_version:selected.version}});
   setSelected(null);setRemoving(false);await list.reload();setNote('Document deleted.');
  } catch(e) {setNote((e as Error).message);} finally {setBusy(false);}
 };
 return <Card title="Document knowledge" aside={<Pill>{list.data?.documents.length ?? 0} visible sources</Pill>}>
  <p className="muted small">Add text or Markdown SOPs and runbooks, then choose Document knowledge when asking. Source text is evidence; it cannot execute actions.</p>
  <ErrorNote message={list.error} />
  {list.data?.documents.length === 0 ? <Empty>No documents are visible in this pack.</Empty> : null}
  <ul className="list">{list.data?.documents.map(d => <li key={d.id}><button className="linklike" onClick={() => open(d.id)}>{d.title}</button><span className="muted small">v{d.version} · {d.visibility}</span></li>)}</ul>
  {admin ? <button className="btn-secondary" onClick={() => {setDraft({...emptyDocument,roles:[]});setEditing(true);setRemoving(false);setNote('');}}>Add document</button> : null}
  {selected && !editing ? <div className="info-note">
   <h3>{selected.title}</h3><p className="muted small">{selected.source || 'Manually supplied source'} · v{selected.version} · {selected.visibility}{selected.tenant ? ` (${selected.tenant})` : ''} · Readers: {selected.roles.join(', ') || 'all permitted roles'}</p>
   <p className="mono small">SHA-256 {selected.sha256}</p><pre className="code">{selected.text}</pre>
   {admin ? <div className="pills"><button className="btn-secondary" onClick={() => {setDraft(selected);setEditing(true);setRemoving(false);}}>Edit document</button><button className="btn-secondary" onClick={() => setRemoving(true)}>Delete document</button></div> : null}
   {removing ? <div className="info-note"><p>Delete this document and all retained text revisions? Existing citations will become unavailable.</p><button className="btn-secondary" disabled={busy} onClick={remove}>Confirm deletion</button><button className="btn-secondary" onClick={() => setRemoving(false)}>Cancel deletion</button></div> : null}
  </div> : null}
  {editing && admin ? <form onSubmit={e => {e.preventDefault();save();}}>
   <label>Document ID<input aria-label="Document ID" value={draft.id} disabled={draft.version > 0} maxLength={80} required onChange={e => setDraft({...draft,id:e.target.value})} /></label>
   <label>Title<input aria-label="Document title" value={draft.title} maxLength={200} required onChange={e => setDraft({...draft,title:e.target.value})} /></label>
   <label>Source reference<input aria-label="Document source" value={draft.source} maxLength={1024} onChange={e => setDraft({...draft,source:e.target.value})} /></label>
   <label>Visibility<select aria-label="Document visibility" value={draft.visibility} onChange={e => setDraft({...draft,visibility:e.target.value as KnowledgeDocument['visibility'],tenant:''})}><option value="provider">Provider only</option><option value="shared">Shared with tenants</option><option value="tenant">One tenant</option></select></label>
   {draft.visibility === 'tenant' ? <label>Tenant<input aria-label="Document tenant" value={draft.tenant ?? ''} required maxLength={40} onChange={e => setDraft({...draft,tenant:e.target.value})} /></label> : null}
   <fieldset><legend>Reader roles (none selected means all permitted roles)</legend>{['viewer','proposer','approver','executor','admin'].map(role => <label key={role}><input type="checkbox" checked={draft.roles.includes(role)} onChange={e => setDraft({...draft,roles:e.target.checked ? [...draft.roles,role] : draft.roles.filter(r => r !== role)})} />{role}</label>)}</fieldset>
   <label>Text or Markdown<textarea aria-label="Document text" value={draft.text ?? ''} required maxLength={131072} rows={10} onChange={e => setDraft({...draft,text:e.target.value})} /></label>
   <button className="primary" type="submit" disabled={busy}>Save document</button><button className="btn-secondary" type="button" onClick={() => setEditing(false)}>Cancel edit</button>
  </form> : null}
  {note ? <p role="status">{note}</p> : null}
 </Card>;
}
