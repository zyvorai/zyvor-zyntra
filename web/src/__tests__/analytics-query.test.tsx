import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import Ask from '../pages/Ask';
import AnalyticsQueryPanel from '../components/AnalyticsQueryPanel';
import { mockApi } from '../test/mock';
import type { AnalyticsQueryResult } from '../api';
HTMLElement.prototype.scrollIntoView = vi.fn();

const result: AnalyticsQueryResult = {
 query: {metrics:['latency'],operation:'trend',window_hours:2,bucket_hours:1},
 start:'2026-10-05T10:00:00Z',end:'2026-10-05T12:00:00Z',method:'Observed samples only.',
 rows:[{id:'latency',name:'API latency',unit:'ms',value:null,samples:2,note:'See bucket means.',sha256:'abcdef',warning:'Current observations are stale.',first:{t:'2026-10-05T10:00:00Z',v:10},last:{t:'2026-10-05T11:00:00Z',v:20},buckets:[{start:'2026-10-05T10:00:00Z',end:'2026-10-05T11:00:00Z',value:10,samples:1},{start:'2026-10-05T11:00:00Z',end:'2026-10-05T12:00:00Z',value:20,samples:1}]}],
};
it('submits analytics scope and shows validated query evidence and buckets',async()=>{
 const calls=mockApi({
  'GET /api/v1/analytics/catalog':{metrics:[{id:'latency',name:'API latency',unit:'ms'}]},
  'GET /api/v1/ai/status':{mode:'heuristic',mutations:'never'},
  'GET /api/v1/ontology/actions':{actions:[]},
  'GET /api/v1/knowledge/documents':{documents:[]},
  'POST /api/v1/ai/ask':{text:'Calculated trend.',intent:'analytics-query',mode:'heuristic',grounding:[],analytics_query:result},
 });
 render(<Ask />);
 fireEvent.change(screen.getByLabelText('Answer source'),{target:{value:'analytics'}});
 fireEvent.click(await screen.findByRole('button',{name:'API latency (latency)'}));
 expect((screen.getByLabelText('Your question') as HTMLInputElement).value).toBe('average latency over the last 24 hours');
 fireEvent.change(screen.getByLabelText('Your question'),{target:{value:'hourly trend latency last 2 hours'}});
 fireEvent.click(screen.getByRole('button',{name:'Ask'}));
 expect(await screen.findByText('Analytics evidence (1 metrics)')).toBeTruthy();
 expect(screen.getByRole('table',{name:'Observed bucket means for latency'})).toBeTruthy();
 expect(screen.getByText('Current observations are stale.')).toBeTruthy();
 expect(screen.getByText('SHA-256 abcdef')).toBeTruthy();
 expect(calls.find(c=>c.method==='POST')?.body).toEqual({question:'hourly trend latency last 2 hours',scope:'analytics'});
});
it('presents missing data as unavailable and preserves the method',()=>{
 render(<AnalyticsQueryPanel result={{...result,rows:[{id:'revenue',name:'Revenue',unit:'USD',value:null,samples:0,note:'No observed samples in this window.'}]}} />);
 expect(screen.getByText(/No observed samples in this window/)).toBeTruthy();
 expect(screen.getByText('Observed samples only.')).toBeTruthy();
 expect(screen.queryByRole('table')).toBeNull();
});
