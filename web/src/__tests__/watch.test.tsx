import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import WatchInbox from '../components/WatchInbox';
import { WhoContext } from '../session';
import { mockApi } from '../test/mock';
import { unnamedControls } from '../test/a11y';
import type { WatchRule, WatchView, WhoAmI } from '../api';
const rule:WatchRule={id:'lat',name:'High latency',metric:'latency',metric_name:'API latency',unit:'ms',operator:'above',threshold:200,for_seconds:300,clear_seconds:60,max_gap_seconds:120,enabled:true,version:1,updated_at:'2026-10-05T12:00:00Z',updated_by:'admin'};
const view:WatchView={rules:[rule],runtime:{lat:{status:'open'}},incidents:[{id:'watch-2',rule,version:1,status:'open',opened_at:'2026-10-05T12:00:00Z',observed_at:'2026-10-05T12:00:00Z',value:250}],events:[]};
const admin:WhoAmI={identity:{subject:'admin',role:'admin',roles:['admin'],method:'password'},auth_required:true};
it('creates an admin watch with version guard and accessible fields',async()=>{
 const calls=mockApi({'GET /api/v1/watches':{...view,rules:[],incidents:[]},'GET /api/v1/analytics/catalog':{metrics:[{id:'latency',name:'API latency',unit:'ms'}]},'PUT /api/v1/watches/lat':rule});
 const {container}=render(<WhoContext.Provider value={admin}><WatchInbox /></WhoContext.Provider>);
 await screen.findByText('No retained watch incidents.');fireEvent.click(screen.getByRole('button',{name:'Add watch rule'}));
 fireEvent.change(screen.getByLabelText('Watch rule ID'),{target:{value:'lat'}});fireEvent.change(screen.getByLabelText('Watch rule name'),{target:{value:'High latency'}});
 fireEvent.change(screen.getByLabelText('Watch threshold'),{target:{value:'200'}});fireEvent.click(screen.getByRole('button',{name:'Save watch rule'}));
 expect(await screen.findByText('Watch rule saved.')).toBeTruthy();expect(unnamedControls(container)).toEqual([]);
 expect(calls.find(c=>c.method==='PUT')?.body).toMatchObject({metric:'latency',threshold:200,for_seconds:300,clear_seconds:60,max_gap_seconds:120,enabled:true,expected_version:0});
});
it('tenant viewers can inspect but cannot acknowledge or edit rules',async()=>{
 mockApi({'GET /api/v1/watches':view,'GET /api/v1/analytics/catalog':{metrics:[]}});
 render(<WhoContext.Provider value={{identity:{subject:'alice',role:'viewer',roles:['viewer'],tenant:'alpha',method:'password'},auth_required:true}}><WatchInbox /></WhoContext.Provider>);
 expect(await screen.findByText('High latency · open · watch-2')).toBeTruthy();
 expect(screen.queryByRole('button',{name:'Add watch rule'})).toBeNull();expect(screen.queryByRole('button',{name:'Acknowledge watch-2'})).toBeNull();expect(screen.queryByRole('button',{name:'Edit lat'})).toBeNull();
});
it('acknowledges with a note and the current incident version',async()=>{
 const calls=mockApi({'GET /api/v1/watches':view,'GET /api/v1/analytics/catalog':{metrics:[]},'POST /api/v1/watch-incidents/watch-2/acknowledge':{...view.incidents[0],version:2,status:'acknowledged'}});
 render(<WhoContext.Provider value={{identity:{subject:'alice',role:'proposer',roles:['proposer'],tenant:'alpha',method:'password'},auth_required:true}}><WatchInbox /></WhoContext.Provider>);
 fireEvent.click(await screen.findByRole('button',{name:'Acknowledge watch-2'}));fireEvent.change(screen.getByLabelText('Acknowledgement note'),{target:{value:'Checking upstream queue'}});fireEvent.click(screen.getByRole('button',{name:'Confirm acknowledgement'}));
 expect(await screen.findByText('Incident acknowledged.')).toBeTruthy();expect(calls.find(c=>c.method==='POST')?.body).toEqual({expected_version:1,note:'Checking upstream queue'});
});
it('surfaces persistence failures and recorded recovery reasons',async()=>{
 mockApi({'GET /api/v1/watches':{...view,persistence_error:'Watch persistence failed; last durable state retained.',incidents:[{...view.incidents[0],status:'resolved',resolved_at:'2026-10-05T12:10:00Z',resolve_reason:'rule changed'}]},'GET /api/v1/analytics/catalog':{metrics:[]}});
 render(<WatchInbox />);expect(await screen.findByText('Watch persistence failed; last durable state retained.')).toBeTruthy();expect(screen.getByText(/Resolved .*rule changed/)).toBeTruthy();
});

it('marks unavailable data without implying an open incident recovered',async()=>{
 mockApi({'GET /api/v1/watches':{...view,runtime:{lat:{status:'unavailable',active_incident:'watch-2'}}},'GET /api/v1/analytics/catalog':{metrics:[]}});
 render(<WatchInbox />);expect(await screen.findByText('Current observations are unavailable; this incident has not been cleared.')).toBeTruthy();
});
