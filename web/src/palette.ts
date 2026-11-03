// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

export interface PaletteItem {
  id: string;
  group: string;
  label: string;
  hint?: string;
  run: () => void;
}

/** score ranks how well query matches text: 0 means no match, higher is better. */
export function score(query: string, text: string): number {
  const q = query.trim().toLowerCase();
  if (!q) return 1;
  const t = text.toLowerCase();
  if (t === q) return 100;
  if (t.startsWith(q)) return 80;
  const word = t.indexOf(' ' + q);
  if (word >= 0) return 60;
  if (t.includes(q)) return 40;
  // Letters in order ("gp" matches "Gaps"): a weak match so typos still land.
  let i = 0;
  for (const ch of t) if (ch === q[i]) i++;
  return i === q.length ? 10 : 0;
}

/** rank filters and orders items for a query, keeping the given order for ties. */
export function rank(items: PaletteItem[], query: string): PaletteItem[] {
  return items
    .map((it, i) => ({ it, i, s: Math.max(score(query, it.label), score(query, `${it.label} ${it.hint ?? ''}`) * 0.5) }))
    .filter((x) => x.s > 0)
    .sort((a, b) => b.s - a.s || a.i - b.i)
    .map((x) => x.it);
}
