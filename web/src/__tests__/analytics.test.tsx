import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import Insights from '../pages/Insights';
import { mockApi } from '../test/mock';

it('shows selected forecasts, backtests, warming states and persistence errors', async () => {
 mockApi({
  'GET /api/v1/ai/insights': {anomalies: [], forecasts: [], analytics: {
   persistent: true, persistence_error: 'disk unavailable', interval_note: 'Empirical errors, not calibrated intervals.', anomalies: [],
   kpis: [
    {kpi: 'sales', name: 'Sales', unit: 'INR', status: 'ready', observations: 10000, hourly_samples: 240, selected: 'daily-seasonal',
     backtests: [{method: 'daily-seasonal', samples: 24, mae: 3, rmse: 4, baseline_mae: 10, error_radius: 6}],
     projections: [{hours: 24, value: 100, low: 94, high: 106, breach: true}]},
    {kpi: 'wait', name: 'Wait', unit: 'min', status: 'warming', reason: 'Need more hourly observations.', observations: 10, hourly_samples: 1, backtests: [], projections: []},
   ],
  }},
  'GET /api/v1/ai/status': {mode:'heuristic', mutations:'never'},
  'GET /api/v1/ai/calibration': {decisions:0,kpis:[],suggestions:[],note:'No decisions.'},
 });
 render(<Insights setPage={() => {}} />);
 expect(await screen.findByText('Seasonal analytics')).toBeTruthy();
 expect(screen.getByText('Persistent history')).toBeTruthy();
 expect(screen.getByText('disk unavailable')).toBeTruthy();
 expect(screen.getByText('Projected miss')).toBeTruthy();
 expect(screen.getByText('Need more hourly observations.')).toBeTruthy();
 expect(screen.getByRole('table', {name: 'Forecast accuracy for Sales'})).toBeTruthy();
 expect(screen.getByRole('table', {name: 'Projections for Sales'})).toBeTruthy();
});

it('does not invent a band for a warming long horizon', async () => {
 mockApi({
  'GET /api/v1/ai/insights': {anomalies:[],forecasts:[],analytics:{persistent:true,interval_note:'Horizon-specific validation',anomalies:[],kpis:[{
   kpi:'sales',name:'Sales',unit:'INR',status:'ready',observations:100,hourly_samples:99,selected:'linear',backtests:[],
   horizon_backtests:[{hours:168,status:'warming',reason:'Need more training history',samples:0,calibration_samples:0,mae:0,baseline_mae:0,coverage:0}],
   projections:[{hours:168,value:500,breach:false,interval_status:'warming'}],
  }]}},
  'GET /api/v1/ai/status':{mode:'heuristic',mutations:'never'},
  'GET /api/v1/ai/calibration':{decisions:0,kpis:[],suggestions:[],note:'No decisions.'},
 });
 render(<Insights setPage={()=>{}} />);
 expect(await screen.findByText('Warming: more history needed')).toBeTruthy();
 expect(screen.getByText('Need more training history')).toBeTruthy();
});
