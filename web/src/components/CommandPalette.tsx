// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type OntObject } from '../api';
import { openObject, visibleGroups, type Page } from '../nav';
import { rank, type PaletteItem } from '../palette';

/** Opens with Cmd/Ctrl-K or "/" (outside a text field) and jumps to any page or object. */
export default function CommandPalette({ setPage, ontology, tenant }: { setPage: (p: Page) => void; ontology?: boolean; tenant?: string }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const [objects, setObjects] = useState<OntObject[]>([]);
  const input = useRef<HTMLInputElement>(null);
  const opener = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      const typing = !!el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable);
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        opener.current = document.activeElement as HTMLElement | null;
        setOpen((o) => !o);
      } else if (e.key === '/' && !typing && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        opener.current = document.activeElement as HTMLElement | null;
        setOpen(true);
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  useEffect(() => {
    if (open) {
      setQuery('');
      setActive(0);
      requestAnimationFrame(() => input.current?.focus());
    } else {
      opener.current?.focus?.();
    }
  }, [open]);

  // Object search: only when the pack has an ontology and the query is long enough to mean something.
  useEffect(() => {
    if (!open || !ontology || query.trim().length < 2) {
      setObjects([]);
      return;
    }
    let live = true;
    const t = setTimeout(() => {
      api<{ objects: OntObject[] }>(`/api/v1/ontology/objects?limit=6&q=${encodeURIComponent(query.trim())}`)
        .then((r) => live && setObjects(r.objects ?? []))
        .catch(() => live && setObjects([]));
    }, 180);
    return () => {
      live = false;
      clearTimeout(t);
    };
  }, [open, query, ontology]);

  const close = () => setOpen(false);
  const pages = useMemo<PaletteItem[]>(
    () =>
      visibleGroups(ontology, tenant).flatMap((g) =>
        g.children
          ? g.children.map((c) => ({ id: `page:${c.page}`, group: g.label, label: c.label, hint: c.blurb, run: () => setPage(c.page) }))
          : [{ id: `page:${g.page}`, group: 'Go to', label: g.label, run: () => setPage(g.page as Page) }],
      ),
    [ontology, tenant, setPage],
  );
  const items = useMemo(() => {
    const found: PaletteItem[] = objects.map((o) => ({
      id: `obj:${o.id}`,
      group: 'Objects',
      label: String(o.props?.name?.v ?? o.id),
      hint: `${o.type} · ${o.id}`,
      run: () => openObject(o.id),
    }));
    return [...rank(pages, query), ...found];
  }, [pages, objects, query]);

  if (!open) return null;

  const choose = (it: PaletteItem | undefined) => {
    if (!it) return;
    close();
    it.run();
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      close();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, items.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      choose(items[active]);
    } else if (e.key === 'Tab') {
      e.preventDefault(); // the dialog is a single focus stop
    }
  };

  let lastGroup = '';
  return (
    <div className="palette-scrim" onMouseDown={(e) => e.target === e.currentTarget && close()}>
      <div className="palette" role="dialog" aria-modal="true" aria-label="Command palette" onKeyDown={onKeyDown}>
        <input
          ref={input}
          role="combobox"
          aria-expanded="true"
          aria-controls="palette-list"
          aria-activedescendant={items[active] ? `palette-${active}` : undefined}
          aria-label="Search pages and objects"
          placeholder="Jump to a page or object"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(0);
          }}
        />
        {items.length === 0 ? (
          <p className="palette-empty">Nothing matches “{query}”.</p>
        ) : (
          <ul id="palette-list" role="listbox" aria-label="Results">
            {items.map((it, i) => {
              const head = it.group !== lastGroup;
              lastGroup = it.group;
              return (
                <li key={it.id} role="presentation">
                  {head ? <div className="palette-group" aria-hidden>{it.group}</div> : null}
                  <button
                    type="button"
                    id={`palette-${i}`}
                    role="option"
                    tabIndex={-1}
                    aria-selected={i === active}
                    onMouseMove={() => setActive(i)}
                    onClick={() => choose(it)}
                  >
                    {it.label}
                    {it.hint ? <small>{it.hint}</small> : null}
                  </button>
                </li>
              );
            })}
          </ul>
        )}
        <p className="palette-hint">
          <kbd>↑</kbd> <kbd>↓</kbd> to move · <kbd>Enter</kbd> to open · <kbd>Esc</kbd> to close
        </p>
      </div>
    </div>
  );
}
