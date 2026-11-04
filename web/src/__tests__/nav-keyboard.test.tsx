import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { type ServedPack } from '../api';
import Nav from '../components/Nav';

const mount = () =>
  render(<Nav page="overview" setPage={() => {}} theme="light" onToggleTheme={() => {}} pulse={null} operator="ann" />);

describe('Nav keyboard', () => {
  it('ArrowDown on a group button opens it and focuses the first item', async () => {
    mount();
    const trigger = screen.getByRole('button', { name: /^Decide/ });
    trigger.focus();
    fireEvent.keyDown(trigger, { key: 'ArrowDown' });
    await waitFor(() => expect(trigger.getAttribute('aria-expanded')).toBe('true'));
    await waitFor(() => {
      const panel = trigger.parentElement!.querySelector('.mega-panel')!;
      expect(panel.contains(document.activeElement)).toBe(true);
    });
  });

  it('Escape closes the phone menu and returns focus to its button', () => {
    mount();
    const open = screen.getByRole('button', { name: 'Open menu' });
    open.focus();
    fireEvent.click(open);
    const close = screen.getByRole('button', { name: 'Close menu' });
    expect(close.getAttribute('aria-expanded')).toBe('true');
    fireEvent.keyDown(document, { key: 'Escape' });
    const again = screen.getByRole('button', { name: 'Open menu' });
    expect(again.getAttribute('aria-expanded')).toBe('false');
    expect(document.activeElement).toBe(again);
  });

  it('offers the pack switcher inside the phone menu as well, and the digest chip keeps its name', () => {
    const packs: ServedPack[] = [
      { id: 'gpu', title: 'GPU cluster', default: true },
      { id: 'shop', title: 'Counter and stock' },
    ];
    const pulse = { gaps: [{}, {}], severity_total: 0.2, pending_approvals: 0 } as never;
    render(<Nav page="overview" setPage={() => {}} theme="light" onToggleTheme={() => {}} pulse={pulse} operator="ann" packs={packs} />);
    expect(screen.getAllByLabelText('Pack')).toHaveLength(1);
    expect(screen.getByRole('button', { name: '2 gaps' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }));
    const selects = screen.getAllByLabelText('Pack');
    expect(selects).toHaveLength(2);
    expect(selects.some((el) => el.closest('.pack-select-sheet'))).toBe(true);
  });
});
