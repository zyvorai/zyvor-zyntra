import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import Ask from '../pages/Ask';
import IncidentInvestigationPanel from '../components/IncidentInvestigation';
import KPIInvestigation, { InvestigationEvidence } from '../components/KPIInvestigation';
import { mockApi } from '../test/mock';
import { unnamedControls } from '../test/a11y';
import type { InvestigationReport } from '../api';
HTMLElement.prototype.scrollIntoView = vi.fn();

const report: InvestigationReport = {
 query:{metric:'latency',window_hours:72,recent_hours:6,max_lag_hours:6,candidates:['queue']},
 target:{id:'latency',name:'API latency',unit:'ms',warning:'Current observations are stale.'},
 start:'2026-10-02T12:00:00Z',end:'2026-10-05T12:00:00Z',sha256:'hour-hash',method:'Hourly change correlation only.',
 caveat:'Exploratory associations, not causes or statistical significance.',candidate_total:1,candidates_truncated:false,
 shift:{status:'shift',direction:'up',reason:'Sustained level movement.',baseline_hours:66,recent_hours:6,recent_start:'2026-10-05T06:00:00Z',baseline_median:100,recent_median:140,delta:40,threshold:5,beyond_threshold:6,required_hours:5},
 hours:[{start:'2026-10-05T11:00:00Z',mean:140,samples:60}],
 comparisons:[{id:'queue',name:'Queue depth',unit:'jobs',status:'associated',correlation:.95,lag_hours:2,pairs:69,tested_lags:7,start:'2026-10-02T15:00:00Z',end:'2026-10-05T11:00:00Z',sha256:'pair-hash',evidence:[{at:'2026-10-05T11:00:00Z',candidate_change:10,target_change:20}]}],
};
const catalog={metrics:[{id:'latency',name:'API latency',unit:'ms'}]};
it('runs a bounded investigation and renders shift, leads and evidence',async()=>{
 const calls=mockApi({'GET /api/v1/analytics/catalog':catalog,'POST /api/v1/analytics/investigate':report});
 const {container}=render(<KPIInvestigation />);
 await screen.findByRole('option',{name:'API latency (latency)'});
 fireEvent.change(screen.getByLabelText('Investigation window hours'),{target:{value:'72'}});
 fireEvent.click(screen.getByRole('button',{name:'Investigate'}));
 expect(await screen.findByText('Sustained shift up')).toBeTruthy();
 expect(screen.getByRole('table',{name:'Strongest exploratory associations for latency (up to 5)'})).toBeTruthy();
 expect(screen.getByText('2h earlier')).toBeTruthy();
 expect(screen.getByText('Current observations are stale.')).toBeTruthy();
 expect(screen.getByText('Exploratory associations, not causes or statistical significance.')).toBeTruthy();
 expect(screen.getByRole('table',{name:'Aligned changes for queue'})).toBeTruthy();
 expect(unnamedControls(container)).toEqual([]);
 expect(calls.find(c=>c.method==='POST')?.body).toEqual({metric:'latency',window_hours:72,recent_hours:6,max_lag_hours:6});
});
it('clears a previous report if the next investigation fails',async()=>{
 let runs=0;
 mockApi({'GET /api/v1/analytics/catalog':catalog,'POST /api/v1/analytics/investigate':()=>++runs===1?report:{status:400,body:{error:'Insufficient baseline window'}}});
 render(<KPIInvestigation />);await screen.findByRole('option',{name:'API latency (latency)'});
 fireEvent.click(screen.getByRole('button',{name:'Investigate'}));await screen.findByText('Sustained shift up');
 fireEvent.click(screen.getByRole('button',{name:'Investigate'}));
 expect(await screen.findByText('Insufficient baseline window')).toBeTruthy();
 expect(screen.queryByText('Sustained shift up')).toBeNull();
});
it('asks from investigation scope and renders computed evidence',async()=>{
 const calls=mockApi({
  'GET /api/v1/analytics/catalog':catalog,'GET /api/v1/ai/status':{mode:'heuristic',mutations:'never'},
  'GET /api/v1/ontology/actions':{actions:[]},'GET /api/v1/knowledge/documents':{documents:[]},
  'POST /api/v1/ai/ask':{text:'Observed shift.',intent:'investigation',mode:'heuristic',grounding:[],investigation:report},
 });
 render(<Ask />);fireEvent.change(screen.getByLabelText('Answer source'),{target:{value:'investigation'}});
 fireEvent.click(await screen.findByRole('button',{name:'API latency (latency)'}));
 expect((screen.getByLabelText('Your question') as HTMLInputElement).value).toBe('investigate latency');
 fireEvent.click(screen.getByRole('button',{name:'Ask'}));
 expect(await screen.findByText('Sustained shift up')).toBeTruthy();
 expect(calls.find(c=>c.method==='POST')?.body).toEqual({question:'investigate latency',scope:'investigation'});
});
it('exports the complete computed report as JSON',async()=>{
 const create=vi.fn((_blob: Blob)=> 'blob:test');const revoke=vi.fn();const OriginalURL=URL;
 vi.stubGlobal('URL',class extends OriginalURL {static createObjectURL=create;static revokeObjectURL=revoke});
 const click=vi.spyOn(HTMLAnchorElement.prototype,'click').mockImplementation(()=>{});
 render(<InvestigationEvidence report={report} />);
 fireEvent.click(screen.getByRole('button',{name:'Download investigation JSON'}));
 expect(click).toHaveBeenCalledOnce();
 const blob=create.mock.calls[0][0] as Blob;
 expect(blob.type).toBe('application/json');
 const text=await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.onerror=reject;reader.readAsText(blob);});
 expect(JSON.parse(text)).toEqual(report);
 await waitFor(()=>expect(revoke).toHaveBeenCalledWith('blob:test'));
});
it('shows missing history and candidate truncation without inventing a result',()=>{
 render(<InvestigationEvidence report={{...report,shift:{...report.shift,status:'warming',reason:'Need more completed hours.',delta:undefined},comparisons:[],hours:[],candidate_total:80,candidates_truncated:true}} />);
 expect(screen.getByText('Need more completed hours.')).toBeTruthy();
 expect(screen.getByText('No eligible candidate reached the exploratory association threshold.')).toBeTruthy();
 expect(screen.getByText('Candidate search capped at 64 of 80 permitted metrics, selected by metric ID.')).toBeTruthy();
 expect(screen.getByText('No usable completed hours.')).toBeTruthy();
 expect(screen.queryByRole('table')).toBeNull();
});

it('loads incident evidence only on request and clears it after a failed rerun',async()=>{
 let runs=0;
 const incident={id:'watch-2',opened_at:'2026-10-05T12:30:00Z',status:'resolved',rule:{version:1}};
 const envelope={incident,generated_at:'2026-10-05T15:00:00Z',context:'Historical context before incident opening; partial opening hour excluded.',report};
 const calls=mockApi({'GET /api/v1/watch-incidents/watch-2/investigation':()=>++runs===1?envelope:{status:409,body:{error:'Incident metric unavailable'}}});
 const {container}=render(<IncidentInvestigationPanel id="watch-2" />);
 expect(calls).toEqual([]);
 fireEvent.click(screen.getByRole('button',{name:'Investigate watch-2'}));
 expect(await screen.findByText(envelope.context)).toBeTruthy();
 expect(screen.getByText('Sustained shift up')).toBeTruthy();
 expect(unnamedControls(container)).toEqual([]);
 fireEvent.click(screen.getByRole('button',{name:'Hide investigation watch-2'}));
 expect(screen.queryByText('Sustained shift up')).toBeNull();
 fireEvent.click(screen.getByRole('button',{name:'Investigate watch-2'}));
 expect(await screen.findByText('Incident metric unavailable')).toBeTruthy();
 expect(screen.queryByText('Sustained shift up')).toBeNull();
});
it('exports incident identity and opening context with the analytical report',async()=>{
 const envelope={incident:{id:'watch-2',opened_at:'2026-10-05T12:30:00Z',rule:{version:1}},report};
 const create=vi.fn((_blob:Blob)=>'blob:incident');const OriginalURL=URL;
 vi.stubGlobal('URL',class extends OriginalURL {static createObjectURL=create;static revokeObjectURL=vi.fn()});
 let filename='';vi.spyOn(HTMLAnchorElement.prototype,'click').mockImplementation(function(this:HTMLAnchorElement){filename=this.download});
 render(<InvestigationEvidence report={report} exportValue={envelope} filename="zyntra-incident-watch-2.json" />);
 fireEvent.click(screen.getByRole('button',{name:'Download investigation JSON'}));
 const blob=create.mock.calls[0][0];
 const text=await new Promise<string>(resolve=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.readAsText(blob)});
 expect(JSON.parse(text)).toEqual(envelope);expect(filename).toBe('zyntra-incident-watch-2.json');
});
