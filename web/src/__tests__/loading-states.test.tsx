import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../api';
import Insights from '../pages/Insights';
import Objects from '../pages/Objects';
import Overview from '../pages/Overview';
import Plan from '../pages/Plan';
import { mockApi } from '../test/mock';

describe('loading states do not claim all-clear', () => {
  it('Overview shows a skeleton, not "Every KPI on target", until the first pulse', () => {
    mockApi({});
    render(<Overview pulse={null} setPage={() => {}} />);
    expect(screen.queryByText('Every KPI on target')).toBeNull();
    expect(screen.queryByText(/No gaps/)).toBeNull();
    expect(screen.queryByText(/Nothing to recommend/)).toBeNull();
    expect(document.querySelector('[aria-busy="true"]')).toBeTruthy();
  });

  it('Insights shows no empty-state text while loading', () => {
    mockApi({});
    render(<Insights setPage={() => {}} />);
    expect(screen.queryByText(/No KPI is more than/)).toBeNull();
    expect(document.querySelector('[aria-busy="true"]')).toBeTruthy();
  });

  it('Insights offers Retry when the request fails', async () => {
    mockApi({ 'GET /api/v1/ai/insights': { status: 500, body: { error: 'boom' } } });
    render(<Insights setPage={() => {}} />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Retry' })).toBeTruthy());
    expect(screen.queryByText(/No KPI is more than/)).toBeNull();
  });
});

describe('skeletons take the shape of the page', () => {
  it('Plan shows stacked cards, Insights two cards, Objects a list beside a detail card', () => {
    mockApi({});
    const plan = render(<Plan setPage={() => {}} />);
    expect(plan.container.querySelectorAll('.skeleton-cards .card').length).toBe(3);
    plan.unmount();
    const ins = render(<Insights setPage={() => {}} />);
    expect(ins.container.querySelectorAll('.skeleton-twin .card').length).toBe(2);
    ins.unmount();
    const obj = render(<Objects />);
    expect(obj.container.querySelectorAll('.skeleton-split .card').length).toBe(2);
    expect(obj.container.querySelector('.page-body')?.hasAttribute('hidden')).toBe(true);
  });
});

describe('network failure copy', () => {
  afterEach(() => vi.unstubAllGlobals());
  it('replaces the browser text with something a person can act on', async () => {
    vi.stubGlobal('fetch', () => Promise.reject(new TypeError('Failed to fetch')));
    await expect(api('/api/v1/x')).rejects.toThrow(/Can't reach the server/);
  });
});
