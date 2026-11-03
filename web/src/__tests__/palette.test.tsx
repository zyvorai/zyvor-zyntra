import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { rank, score, type PaletteItem } from '../palette';
import CommandPalette from '../components/CommandPalette';
import { mockApi } from '../test/mock';
import { unnamedControls } from '../test/a11y';

const item = (label: string, hint = ''): PaletteItem => ({ id: label, group: 'g', label, hint, run: () => undefined });

describe('palette ranking', () => {
  it('prefers exact, then prefix, then word, then substring, then in-order letters', () => {
    expect(score('gaps', 'Gaps')).toBeGreaterThan(score('gap', 'Gaps'));
    expect(score('gap', 'Gaps')).toBeGreaterThan(score('ap', 'Gaps'));
    expect(score('gp', 'Gaps')).toBeGreaterThan(0);
    expect(score('zz', 'Gaps')).toBe(0);
  });

  it('keeps the given order for ties and drops non-matches', () => {
    const items = [item('Plan'), item('Approvals'), item('Audit')];
    expect(rank(items, '').map((i) => i.label)).toEqual(['Plan', 'Approvals', 'Audit']);
    expect(rank(items, 'au').map((i) => i.label)).toEqual(['Audit']);
    expect(rank(items, 'nothing')).toEqual([]);
  });

  it('matches on the blurb too, but ranks a label match first', () => {
    const items = [item('Alpha', 'ranked by risk'), item('Risk board')];
    expect(rank(items, 'risk').map((i) => i.label)).toEqual(['Risk board', 'Alpha']);
  });
});

describe('CommandPalette', () => {
  it('opens on Ctrl-K, filters pages, and navigates on Enter', async () => {
    mockApi({});
    const setPage = vi.fn();
    const { container } = render(<CommandPalette setPage={setPage} />);
    expect(screen.queryByRole('dialog')).toBeNull();
    fireEvent.keyDown(document, { key: 'k', ctrlKey: true });
    const box = await screen.findByRole('combobox', { name: 'Search pages and objects' });
    fireEvent.change(box, { target: { value: 'audit' } });
    expect(screen.getByRole('option', { name: /Audit/ })).toBeTruthy();
    expect(screen.queryByRole('option', { name: /Gaps/ })).toBeNull();
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Enter' });
    expect(setPage).toHaveBeenCalledWith('audit');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(unnamedControls(container)).toEqual([]);
  });

  it('closes on Escape and says when nothing matches', async () => {
    mockApi({});
    render(<CommandPalette setPage={() => undefined} />);
    fireEvent.keyDown(document, { key: '/' });
    const box = await screen.findByRole('combobox');
    fireEvent.change(box, { target: { value: 'qqqq' } });
    expect(screen.getByText(/Nothing matches/)).toBeTruthy();
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('ignores "/" while typing in a field', () => {
    mockApi({});
    render(
      <>
        <input aria-label="field" />
        <CommandPalette setPage={() => undefined} />
      </>,
    );
    fireEvent.keyDown(screen.getByLabelText('field'), { key: '/' });
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});
