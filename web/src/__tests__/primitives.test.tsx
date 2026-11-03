import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Button, Empty, ErrorNote, MetricBand, PageSkeleton, Reveal } from '../components/ui';
import { unnamedControls } from '../test/a11y';

describe('ui primitives', () => {
  it('ErrorNote shows nothing without a message and offers Retry when asked', () => {
    const { container, rerender } = render(<ErrorNote message="" />);
    expect(container.textContent).toBe('');
    const retry = vi.fn();
    rerender(<ErrorNote message="boom" onRetry={retry} />);
    expect(screen.getByRole('alert').textContent).toContain('boom');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it('ErrorNote without a retry is still a plain alert', () => {
    render(<ErrorNote message="nope" />);
    expect(screen.getByRole('alert').textContent).toBe('nope');
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('Empty keeps its plain form and gains an icon with an action', () => {
    const { container, rerender } = render(<Empty>Nothing here</Empty>);
    expect(container.querySelector('p.empty')?.textContent).toBe('Nothing here');
    expect(container.querySelector('svg')).toBeNull();
    rerender(<Empty action={<Button>Add one</Button>}>Nothing here</Empty>);
    expect(container.querySelector('svg')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Add one' })).toBeTruthy();
  });

  it('PageSkeleton is busy, announces Loading once, and hides its blocks', () => {
    const { container } = render(<PageSkeleton />);
    expect(container.querySelector('[aria-busy="true"]')).toBeTruthy();
    expect(screen.getAllByText('Loading')).toHaveLength(1);
    expect(container.querySelectorAll('.skeleton[aria-hidden]').length).toBeGreaterThan(3);
  });

  it('Reveal shows its content at once when IntersectionObserver is missing', () => {
    const { container } = render(<Reveal>hello</Reveal>);
    expect(container.querySelector('.zv-reveal--visible')?.textContent).toBe('hello');
  });

  it('MetricBand renders a label and a value per item, and Button has a name', () => {
    const { container } = render(
      <>
        <MetricBand items={[{ label: 'Gaps', value: 3 }, { label: 'Sources', value: '4/4' }]} />
        <Button variant="primary">Go</Button>
      </>,
    );
    expect(screen.getByText('Gaps').nextSibling?.textContent).toBe('3');
    expect(container.querySelector('button.btn-primary')?.getAttribute('type')).toBe('button');
    expect(unnamedControls(container)).toEqual([]);
  });
});
