import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import Objects from '../pages/Objects';
import Scenarios from '../pages/Scenarios';
import Workflows from '../pages/Workflows';
import { mockApi } from '../test/mock';

describe('Objects on a pack without an ontology', () => {
  it('shows one calm empty state and makes no ontology requests', async () => {
    const calls = mockApi({});
    render(<Objects ontology={false} />);
    expect(screen.getByRole('heading', { level: 1, name: 'Objects' })).toBeTruthy();
    expect(screen.getByText(/This pack has no ontology/)).toBeTruthy();
    expect(screen.getByText('zyntra ontology suggest')).toBeTruthy();
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.queryByRole('tablist')).toBeNull();
    await new Promise((r) => setTimeout(r, 50));
    expect(calls).toHaveLength(0);
  });

  it('swaps to the same empty state when the server says there is no ontology', async () => {
    mockApi({
      'GET /api/v1/ontology/schema': { status: 404, body: { error: 'this pack has no ontology' } },
      'GET /api/v1/ontology/objects': { status: 404, body: { error: 'this pack has no ontology' } },
    });
    render(<Objects />);
    await waitFor(() => expect(screen.getByText(/so there are no business objects to show/)).toBeTruthy());
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.queryByRole('tablist')).toBeNull();
  });
});

describe('Workflows and Scenarios on a pack without an ontology', () => {
  it('Workflows shows the empty state and makes no requests', async () => {
    const calls = mockApi({});
    render(<Workflows ontology={false} />);
    expect(screen.getByRole('heading', { level: 1, name: 'Workflows' })).toBeTruthy();
    expect(screen.getByText(/so there are no workflows to show/)).toBeTruthy();
    expect(screen.queryByRole('alert')).toBeNull();
    await new Promise((r) => setTimeout(r, 50));
    expect(calls).toHaveLength(0);
  });

  it('Workflows swaps to it when the server answers "no ontology"', async () => {
    mockApi({ 'GET /api/v1/ontology/schema': { status: 404, body: { error: 'this pack has no ontology' } } });
    render(<Workflows />);
    await waitFor(() => expect(screen.getByText(/so there are no workflows to show/)).toBeTruthy());
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('Scenarios shows the empty state, with no form, and makes no requests', async () => {
    const calls = mockApi({});
    render(<Scenarios ontology={false} />);
    expect(screen.getByRole('heading', { level: 1, name: 'Scenarios' })).toBeTruthy();
    expect(screen.getByText(/so there are no scenarios to show/)).toBeTruthy();
    expect(screen.queryByLabelText('Scenario name')).toBeNull();
    await new Promise((r) => setTimeout(r, 50));
    expect(calls).toHaveLength(0);
  });
});
