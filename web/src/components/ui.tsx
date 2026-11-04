import { useEffect, useRef, useState } from 'react';
import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { Inbox } from 'lucide-react';
import { fmt, unitOf, type KPI } from '../api';

export function PageHero({
  eyebrow,
  title,
  lede,
  tint,
  actions,
  size,
}: {
  size?: 'display';
  eyebrow?: string;
  title: string;
  lede?: ReactNode;
  tint?: 'green' | 'amber' | 'purple' | 'red';
  actions?: ReactNode;
}) {
  return (
    <header className={`page-hero${size === 'display' ? ' display' : ''}${tint ? ` hero-tint-${tint}` : ''}`}>
      {eyebrow ? <p className="eyebrow">{eyebrow}</p> : null}
      <h1>{title}</h1>
      {lede ? <p>{lede}</p> : null}
      {actions ? <div className="hero-actions">{actions}</div> : null}
    </header>
  );
}

export function Card({ title, aside, children, className = '' }: { title?: ReactNode; aside?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`card ${className}`.trim()}>
      {title || aside ? (
        <div className="card-head">
          {title ? <h2>{title}</h2> : <span />}
          {aside}
        </div>
      ) : null}
      {children}
    </section>
  );
}

export function Stat({ label, value, sub, tone }: { label: string; value: ReactNode; sub?: ReactNode; tone?: 'ok' | 'warn' | 'bad' | 'info' }) {
  return (
    <div className={`stat${tone ? ` stat-${tone}` : ''}`}>
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
      {sub ? <span className="stat-sub">{sub}</span> : null}
    </div>
  );
}

export function Pill({ tone = 'neutral', children }: { tone?: 'ok' | 'warn' | 'bad' | 'info' | 'neutral' | 'purple'; children: ReactNode }) {
  return <span className={`pill pill-${tone}`}>{children}</span>;
}

export const riskTone = (r?: string) => (r === 'high' ? 'bad' : r === 'medium' ? 'warn' : 'ok');

export function Empty({ children, action }: { children: ReactNode; action?: ReactNode }) {
  if (!action) return <p className="empty">{children}</p>;
  return (
    <div className="empty-state">
      <Inbox size={28} aria-hidden />
      <p className="empty">{children}</p>
      {action}
    </div>
  );
}

export function ErrorNote({ message, onRetry }: { message: string; onRetry?: () => void }) {
  if (!message) return null;
  if (!onRetry)
    return (
      <p className="error-note" role="alert">
        {message}
      </p>
    );
  return (
    <div className="error-note with-retry" role="alert">
      <span>{message}</span>
      <Button size="sm" onClick={onRetry}>
        Retry
      </Button>
    </div>
  );
}

export function Button({
  variant = 'quiet',
  size,
  className = '',
  type = 'button',
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'quiet' | 'danger'; size?: 'sm' }) {
  return <button type={type} className={`btn btn-${variant}${size ? ` btn-${size}` : ''} ${className}`.trim()} {...rest} />;
}

/** A shimmering placeholder; decorative, so hidden from assistive tech. */
export function Skeleton({ className = 'skeleton-line' }: { className?: string }) {
  return <span className={`skeleton ${className}`} aria-hidden />;
}

/** Placeholder for a page body while its first request is in flight. */
export function PageSkeleton({ stats = true, rows = 4 }: { stats?: boolean; rows?: number }) {
  return (
    <div aria-busy="true" aria-live="polite">
      <span className="sr-only">Loading</span>
      {stats ? (
        <div className="skeleton-stats" aria-hidden>
          <Skeleton className="" />
          <Skeleton className="" />
          <Skeleton className="" />
        </div>
      ) : null}
      <div className="card" aria-hidden>
        {Array.from({ length: rows }, (_, i) => (
          <Skeleton key={i} className={i % 3 === 2 ? 'skeleton-line short' : 'skeleton-line'} />
        ))}
      </div>
    </div>
  );
}

/**
 * Wraps a table so it scrolls sideways and keeps its header in view. On a phone
 * (see story.css) each row becomes a labelled card; for that every cell is
 * given its column's header text as data-label, and the table roles are written
 * out so assistive tech still reads a table once the layout is no longer one.
 */
export function TableScroll({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    const label = () => {
      root.querySelectorAll('table').forEach((t) => {
        t.setAttribute('role', 'table');
        const heads = Array.from(t.querySelectorAll('thead th')).map((h) => (h.textContent ?? '').trim());
        t.querySelectorAll('thead, tbody, tfoot').forEach((g) => g.setAttribute('role', 'rowgroup'));
        t.querySelectorAll('tr').forEach((r) => {
          r.setAttribute('role', 'row');
          Array.from(r.children).forEach((c, i) => {
            if (c.tagName === 'TH') c.setAttribute('role', 'columnheader');
            else {
              c.setAttribute('role', 'cell');
              if (heads[i] && c.getAttribute('data-label') !== heads[i]) c.setAttribute('data-label', heads[i]);
            }
          });
        });
      });
    };
    label();
    if (typeof MutationObserver === 'undefined') return;
    const mo = new MutationObserver(label);
    mo.observe(root, { childList: true, subtree: true });
    return () => mo.disconnect();
  }, []);
  return (
    <div className="table-scroll" ref={ref}>
      {children}
    </div>
  );
}

/** Fades a block in once it scrolls into view; always visible without IntersectionObserver. */
export function Reveal({ children, className = '', delay = 0 }: { children: ReactNode; className?: string; delay?: number }) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    const node = ref.current;
    if (!node || typeof IntersectionObserver === 'undefined') {
      setVisible(true);
      return;
    }
    const io = new IntersectionObserver(
      ([e]) => {
        if (e.isIntersecting) {
          setVisible(true);
          io.disconnect();
        }
      },
      { threshold: 0.12 },
    );
    io.observe(node);
    return () => io.disconnect();
  }, []);
  return (
    <div ref={ref} className={`zv-reveal${visible ? ' zv-reveal--visible' : ''} ${className}`.trim()} style={delay ? { transitionDelay: `${delay}ms` } : undefined}>
      {children}
    </div>
  );
}

/** A hairline-ruled strip of large figures, for the top of a landing page. */
export function MetricBand({ items }: { items: { label: string; value: ReactNode }[] }) {
  return (
    <div className="metric-band">
      {items.map((m) => (
        <div key={m.label}>
          <span>{m.label}</span>
          <b>{m.value}</b>
        </div>
      ))}
    </div>
  );
}

export function Sparkline({ points, target, width = 140, height = 36 }: { points: number[]; target?: number; width?: number; height?: number }) {
  if (points.length < 2) return <svg width={width} height={height} className="spark" aria-hidden />;
  const vals = target !== undefined ? [...points, target] : points;
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const span = max - min || 1;
  const x = (i: number) => (i / (points.length - 1)) * (width - 2) + 1;
  const y = (v: number) => height - 2 - ((v - min) / span) * (height - 4);
  const d = points.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('');
  return (
    <svg width={width} height={height} className="spark" aria-hidden>
      {target !== undefined ? <line x1={0} x2={width} y1={y(target)} y2={y(target)} className="spark-target" /> : null}
      <path d={d} className="spark-line" />
    </svg>
  );
}

export function kpiMet(k: KPI): boolean | null {
  if (k.target === undefined || k.target === null) return null;
  return k.direction === 'lower' ? k.value <= k.target : k.value >= k.target;
}

export function KpiValue({ k }: { k: KPI }) {
  return (
    <>
      {fmt(k.value)}
      {unitOf(k) ? <small className="unit"> {unitOf(k)}</small> : null}
    </>
  );
}

export function Meter({ value }: { value: number }) {
  const w = Math.max(2, Math.min(100, value * 100));
  return (
    <span className="meter" aria-hidden>
      <span style={{ width: `${w}%` }} className={value > 0.5 ? 'bad' : value > 0.15 ? 'warn' : 'ok'} />
    </span>
  );
}

export function OwnerFilter({ owners, value, onChange }: { owners?: string[]; value: string; onChange: (o: string) => void }) {
  if (!owners?.length && !value) return null;
  return (
    <label className="owner-filter">
      <span className="muted small">Owner</span>
      <select value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">Everyone</option>
        {(owners ?? []).map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
        {value && !owners?.includes(value) ? <option value={value}>{value}</option> : null}
      </select>
    </label>
  );
}
