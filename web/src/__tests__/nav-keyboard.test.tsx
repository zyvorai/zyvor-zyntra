import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
});
