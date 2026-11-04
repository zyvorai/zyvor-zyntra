import { useEffect, useRef, useState } from 'react';
import type { Theme } from '../theme';
import { visibleGroups, type Page } from '../nav';
import type { Pulse, ServedPack } from '../api';
import { getPack, setPack } from '../pack';

const OPEN_DELAY_MS = 120;
const CLOSE_DELAY_MS = 450;

export default function Nav({
  page,
  setPage,
  theme,
  onToggleTheme,
  onLogout,
  pulse,
  operator,
  ontology,
  tenant,
  packs,
}: {
  page: Page;
  setPage: (p: Page) => void;
  theme: Theme;
  onToggleTheme: () => void;
  onLogout?: () => void;
  pulse: Pulse | null;
  operator: string;
  ontology?: boolean;
  tenant?: string;
  /** Every pack this instance serves; the switcher shows only when there are several. */
  packs?: ServedPack[];
}) {
  const [openGroup, setOpenGroup] = useState<string | null>(null);
  const [sheet, setSheet] = useState(false);
  const openTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const navRef = useRef<HTMLElement | null>(null);
  const menuBtnRef = useRef<HTMLButtonElement | null>(null);
  const triggerRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  const clearTimers = () => {
    if (openTimer.current) clearTimeout(openTimer.current);
    if (closeTimer.current) clearTimeout(closeTimer.current);
    openTimer.current = null;
    closeTimer.current = null;
  };
  const scheduleOpen = (label: string) => {
    clearTimers();
    openTimer.current = setTimeout(() => setOpenGroup(label), OPEN_DELAY_MS);
  };
  const scheduleClose = () => {
    clearTimers();
    closeTimer.current = setTimeout(() => setOpenGroup(null), CLOSE_DELAY_MS);
  };
  const toggleGroup = (label: string) => {
    clearTimers();
    setOpenGroup((cur) => (cur === label ? null : label));
  };

  useEffect(() => () => clearTimers(), []);

  // On a narrow screen the link row scrolls sideways; keep the current section
  // in view so the underline is never hidden off the edge.
  useEffect(() => {
    const active = navRef.current?.querySelector<HTMLElement>('.navlinks .active');
    active?.scrollIntoView?.({ inline: 'center', block: 'nearest' });
    setSheet(false);
  }, [page]);

  // Arrow keys move within an open panel, and from a group button into it.
  const focusItem = (panel: HTMLElement | null, to: 'first' | 'last' | 'next' | 'prev') => {
    const items = Array.from(panel?.querySelectorAll<HTMLButtonElement>('.mega-grid button') ?? []);
    if (!items.length) return;
    const i = items.indexOf(document.activeElement as HTMLButtonElement);
    const n = to === 'first' ? 0 : to === 'last' ? items.length - 1 : to === 'next' ? (i + 1) % items.length : (i - 1 + items.length) % items.length;
    items[n].focus();
  };
  const onPanelKey = (e: React.KeyboardEvent<HTMLElement>) => {
    const map: Record<string, 'first' | 'last' | 'next' | 'prev'> = { ArrowDown: 'next', ArrowRight: 'next', ArrowUp: 'prev', ArrowLeft: 'prev', Home: 'first', End: 'last' };
    const to = map[e.key];
    if (!to) return;
    e.preventDefault();
    focusItem(e.currentTarget, to);
  };
  const onTriggerKey = (e: React.KeyboardEvent<HTMLButtonElement>, label: string) => {
    if (e.key !== 'ArrowDown') return;
    e.preventDefault();
    clearTimers();
    setOpenGroup(label);
    // The panel fades in with a visibility transition, and a browser ignores
    // focus() on a hidden element, so retry for a few frames until it lands.
    const panel = e.currentTarget.parentElement?.querySelector<HTMLElement>('.mega-panel') ?? null;
    const tryFocus = (left: number) => {
      focusItem(panel, 'first');
      if (left > 0 && panel && !panel.contains(document.activeElement)) requestAnimationFrame(() => tryFocus(left - 1));
    };
    requestAnimationFrame(() => tryFocus(12));
  };

  // On a phone the menu is a full-height sheet: Escape closes it and hands
  // focus back to the button that opened it.
  useEffect(() => {
    if (!sheet) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      setSheet(false);
      menuBtnRef.current?.focus();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [sheet]);

  useEffect(() => {
    if (!openGroup) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      const label = openGroup;
      setOpenGroup(null);
      triggerRefs.current[label]?.focus();
    };
    const onPointerDown = (e: MouseEvent) => {
      if (navRef.current && !navRef.current.contains(e.target as Node)) setOpenGroup(null);
    };
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('mousedown', onPointerDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('mousedown', onPointerDown);
    };
  }, [openGroup]);

  // The pack switcher shows only when one instance serves several packs. On a
  // phone the bar has no room for it, so the same control also sits in the menu sheet.
  const packSelect = (className: string) =>
    packs && packs.length > 1 ? (
      <select
        className={className}
        aria-label="Pack"
        value={packs.some((p) => p.id === getPack()) ? getPack() : (packs.find((p) => p.default) ?? packs[0]).id}
        onChange={(e) => {
          // Every cached answer belongs to the old pack, so start the console afresh.
          setPack(e.target.value);
          window.location.hash = '/overview';
          window.location.reload();
        }}
      >
        {packs.map((p) => (
          <option key={p.id} value={p.id}>
            {p.title || p.id}
          </option>
        ))}
      </select>
    ) : null;

  const gaps = pulse?.gaps?.length ?? 0;
  const pending = pulse?.pending_approvals ?? 0;
  const chipTone = pulse && pulse.severity_total > 0.5 ? 'critical' : gaps > 0 ? 'warning' : '';

  return (
    <nav className={`nav${sheet ? ' sheet-open' : ''}`} aria-label="Global" ref={navRef}>
      <div className="nav-inner">
        <button type="button" className="brand" onClick={() => setPage(tenant ? 'servicelevels' : 'overview')} aria-label="Zyntra home">
          <img src="/zyvor-logomark.svg" alt="" className="brand-mark" aria-hidden />
          Zyntra
        </button>
        <button
          type="button"
          ref={menuBtnRef}
          className="theme-toggle nav-menu-btn"
          aria-label={sheet ? 'Close menu' : 'Open menu'}
          aria-expanded={sheet}
          onClick={() => setSheet((v) => !v)}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
            {sheet ? <path d="M6 6l12 12M18 6L6 18" strokeLinecap="round" /> : <path d="M4 7h16M4 12h16M4 17h16" strokeLinecap="round" />}
          </svg>
        </button>
        <div className="navlinks">
          {visibleGroups(ontology, tenant).map((g) =>
            g.children ? (
              <div key={g.label} className="navgroup" onMouseEnter={() => scheduleOpen(g.label)} onMouseLeave={scheduleClose}>
                <button
                  type="button"
                  ref={(el) => {
                    triggerRefs.current[g.label] = el;
                  }}
                  className={g.children.some((c) => c.page === page) ? 'active' : ''}
                  aria-haspopup="true"
                  aria-expanded={openGroup === g.label}
                  onClick={() => toggleGroup(g.label)}
                  onKeyDown={(e) => onTriggerKey(e, g.label)}
                >
                  {g.label}
                  {g.label === 'Act' && pending > 0 ? <span className="nav-badge">{pending}</span> : null}
                </button>
                <div
                  className={`mega-panel${openGroup === g.label ? ' open' : ''}`}
                  role="region"
                  aria-label={g.label}
                  onMouseEnter={() => scheduleOpen(g.label)}
                  onMouseLeave={scheduleClose}
                  onKeyDown={onPanelKey}
                >
                  <div className="mega-grid">
                    {g.children.map((c) => (
                      <button
                        key={c.page}
                        type="button"
                        className={page === c.page ? 'active' : ''}
                        aria-current={page === c.page ? 'page' : undefined}
                        onClick={() => {
                          setPage(c.page);
                          setOpenGroup(null);
                        }}
                      >
                        <span className="mega-link-label">{c.label}</span>
                        <span className="mega-link-blurb">{c.blurb}</span>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            ) : (
              <button
                key={g.page}
                type="button"
                className={page === g.page ? 'active' : ''}
                aria-current={page === g.page ? 'page' : undefined}
                onClick={() => setPage(g.page as Page)}
              >
                {g.label}
              </button>
            )
          )}
          {sheet && packs && packs.length > 1 ? (
            <div className="pack-select-sheet">
              <span aria-hidden>Pack</span>
              {packSelect('pack-select')}
            </div>
          ) : null}
        </div>
        <div className="nav-actions">
          <button
            type="button"
            className="theme-toggle"
            aria-label="Search pages and objects"
            title="Search (Ctrl-K)"
            onClick={() => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, bubbles: true }))}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
              <circle cx="11" cy="11" r="6.5" />
              <path d="M16 16l4.5 4.5" strokeLinecap="round" />
            </svg>
          </button>
          {packSelect('pack-select pack-select-bar')}
          {pulse ? (
            <button
              type="button"
              className={`digest-chip ${chipTone}`}
              aria-label={gaps === 0 ? 'On target' : `${gaps} gap${gaps === 1 ? '' : 's'}`}
              onClick={() => setPage('gaps')}
              title={`Total severity ${(pulse.severity_total * 100).toFixed(1)}%`}
            >
              <span className={`dot ${chipTone || 'ok'}`} aria-hidden />
              <span className="digest-label">{gaps === 0 ? 'On target' : `${gaps} gap${gaps === 1 ? '' : 's'}`}</span>
            </button>
          ) : null}
          {onLogout ? (
            <button type="button" className="theme-toggle" onClick={onLogout} aria-label={`Log out ${operator}`} title={`Log out ${operator}`}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                <path d="M15 4H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h8" strokeLinecap="round" strokeLinejoin="round" />
                <path d="M10 12h11m0 0-3.5-3.5M21 12l-3.5 3.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
          ) : null}
          <button
            type="button"
            className="theme-toggle"
            onClick={onToggleTheme}
            aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            title={theme === 'dark' ? 'Light' : 'Dark'}
          >
            {theme === 'dark' ? (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                <circle cx="12" cy="12" r="4" />
                <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
              </svg>
            ) : (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                <path d="M21 14.5A8.5 8.5 0 1 1 11.5 3a7 7 0 0 0 9.5 11.5z" />
              </svg>
            )}
          </button>
        </div>
      </div>
    </nav>
  );
}
