import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import Planning from '../pages/Planning';
import { mockApi } from '../test/mock';

it('submits portfolio constraints and exposes blocked and incomplete estimates', async () => {
 const calls = mockApi({'POST /api/v1/planning/portfolios': {evaluated: 1, truncated: true, model_version:'v1', note:'Bounded search', candidates:[{
  actions:['scale'], status:'blocked', reasons:['budget cannot be checked'], score:0.25, resources:{},
  finance:{complete:false,missing:['scale'],currency:'INR',cost:0,monthly_net:0,net_12_months:0,cost_of_delay_30_days:0,benefits:[],assumptions:[]},simulation:{kpis:[]},
 }]}});
 render(<Planning />);
 fireEvent.change(screen.getByLabelText('Implementation budget'), {target:{value:'100'}});
 fireEvent.click(screen.getByRole('button',{name:'Compare portfolios'}));
 expect(await screen.findByText('Search limit reached')).toBeTruthy();
 expect(screen.getByText('budget cannot be checked')).toBeTruthy();
 expect(screen.getByText(/Financial data missing for scale/)).toBeTruthy();
 expect(calls[0].body).toMatchObject({budget:100,max_actions:3,max_candidates:2000});
});
it('rejects malformed capacity before making a request',async()=>{
 const calls=mockApi({});render(<Planning />);
 fireEvent.change(screen.getByLabelText('Resource capacities'),{target:{value:'[]'}});
 fireEvent.click(screen.getByRole('button',{name:'Compare portfolios'}));
 expect(await screen.findByText('Capacity must be a JSON object.')).toBeTruthy();expect(calls).toHaveLength(0);
});
