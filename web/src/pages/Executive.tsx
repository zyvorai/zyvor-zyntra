// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import { useState } from 'react';
import { api } from '../api';
import { useApi, withOwner } from '../hooks';
import { Card, Empty, ErrorNote, PageHero, Pill } from '../components/ui';

type Audience = 'leadership' | 'ceo' | 'cto' | 'marketing';

interface InboxItem {
  id: string;
  question: string;
  owner: string;
  due?: string;
  rank: number;
  severity: number;
  why: string;
  next_step: string;
  blocked?: boolean;
  stale?: string[];
}

interface Inbox {
  audience: string;
  items: InboxItem[];
  blocked: number;
  stale_inputs: number;
}

interface Evidence {
  kpi: string;
  name: string;
  owner?: string;
  value: number;
  target?: number;
  unit?: string;
  severity: number;
  fresh: string;
  source: string;
  note?: string;
}

interface Option {
  id: string;
  name: string;
  kind: string;
  rank?: number;
  status: string;
  risk?: string;
  improvement: number;
  pessimistic: number;
  blocked?: string[];
  note?: string;
}

interface Brief {
  id: string;
  question: string;
  owner: string;
  due?: string;
  why?: string;
  evidence: Evidence[];
  options: Option[];
  uncertainty: string[];
  conflicts?: { kind: string; summary: string; owners: string[] }[];
  next_step: { kind: string; summary: string; ready: boolean; action?: string };
  narrative: string;
}

interface Review {
  proposal_id: string;
  name: string;
  age_days: number;
  window: string;
  due: boolean;
  verdict: string;
  hit_rate: number;
  follow_up: string;
}

const audiences: { id: Audience; label: string }[] = [
  { id: 'leadership', label: 'Leadership' },
  { id: 'ceo', label: 'CEO' },
  { id: 'cto', label: 'CTO' },
  { id: 'marketing', label: 'Marketing' },
];

const kindTone = { action: 'info', experiment: 'purple', postpone: 'warn', nothing: 'neutral' } as const;

function num(n: number) {
  if (!Number.isFinite(n)) return '—';
  if (Math.abs(n) >= 100000) return new Intl.NumberFormat('en-IN', { maximumFractionDigits: 0 }).format(n);
  return n.toFixed(2);
}

export default function Executive() {
  const [audience, setAudience] = useState<Audience>('ceo');
  const [decision, setDecision] = useState('spend-or-hire');
  const inbox = useApi<Inbox>(`/api/v1/executive/inbox?audience=${audience}`, 15000);
  const brief = useApi<Brief>(`/api/v1/executive/brief?audience=${audience}&decision=${encodeURIComponent(decision)}`, 15000);
  const reviews = useApi<{ reviews: Review[] }>('/api/v1/executive/reviews', 20000);
  const [probe, setProbe] = useState<string>('');
  const [note, setNote] = useState('');

  const open = (id: string) => {
    setDecision(id);
    window.location.hash = `/executive/${encodeURIComponent(id)}`;
  };

  const runProbe = async (action: string) => {
    setNote('');
    try {
      const res = await api<{ assumptions: { assumption: string; statement: string; flips: boolean; top_before?: string; top_after?: string }[] }>(
        withOwner(`/api/v1/executive/sensitivity?action=${encodeURIComponent(action)}`, ''),
      );
      const lines = (res.assumptions ?? []).map((a) => `${a.assumption}: ${a.flips ? `flips ${a.top_before} → ${a.top_after}` : 'does not flip the top action'}`);
      setProbe(lines.join('\n') || 'No assumptions on this pack.');
    } catch (e) {
      setNote((e as Error).message);
    }
  };

  return (
    <>
      <PageHero
        eyebrow="Zyntra Executive"
        title="Know what needs a decision"
        lede="Compare the options, including postpone and do nothing. The simulator ranks. This page only shows the brief, the constraint, and whether the last decision matched the forecast."
        tint="purple"
        actions={
          <div className="row">
            {audiences.map((a) => (
              <button key={a.id} className={audience === a.id ? 'btn' : 'btn ghost'} onClick={() => setAudience(a.id)}>
                {a.label}
              </button>
            ))}
          </div>
        }
      />
      {inbox.error ? <ErrorNote message={inbox.error} /> : null}
      {note ? <ErrorNote message={note} /> : null}
      <div className="grid-2">
        <Card title="Decision inbox" aside={<Pill tone={inbox.data?.stale_inputs ? 'warn' : 'ok'}>{inbox.data?.stale_inputs ? `${inbox.data.stale_inputs} stale` : 'inputs usable'}</Pill>}>
          {(inbox.data?.items ?? []).length === 0 ? <Empty>No decision needs this audience.</Empty> : null}
          <ul className="list">
            {(inbox.data?.items ?? []).map((item) => (
              <li key={item.id}>
                <button className="linklike" onClick={() => open(item.id)}>
                  {item.rank}. {item.question}
                </button>
                <div className="muted">
                  {item.owner}
                  {item.due ? ` · due ${item.due}` : ''} · {item.next_step}
                </div>
              </li>
            ))}
          </ul>
        </Card>
        <Card title="After approval" aside={<span className="muted">30 / 60 / 90 days</span>}>
          {(reviews.data?.reviews ?? []).length === 0 ? <Empty>No recorded decision to review yet.</Empty> : null}
          <ul className="list">
            {(reviews.data?.reviews ?? []).slice(0, 5).map((r) => (
              <li key={r.proposal_id}>
                <strong>{r.name || r.proposal_id}</strong>
                <div className="muted">
                  day {r.age_days} · window {r.window} · {r.verdict} · hit {(r.hit_rate * 100).toFixed(0)}%
                </div>
                <div>{r.follow_up}</div>
              </li>
            ))}
          </ul>
        </Card>
      </div>
      {brief.data ? (
        <>
          <Card
            title={brief.data.question}
            aside={<Pill tone={brief.data.next_step.ready ? 'ok' : 'warn'}>{brief.data.next_step.kind}</Pill>}
          >
            <p>{brief.data.why}</p>
            <p className="muted">
              Owner {brief.data.owner}
              {brief.data.due ? ` · due ${brief.data.due}` : ''}
            </p>
            <p>{brief.data.narrative}</p>
          </Card>
          <Card title="Options" aside={<span className="muted">including postpone and do nothing</span>}>
            <table className="data">
              <thead>
                <tr>
                  <th>Option</th>
                  <th>Kind</th>
                  <th>Status</th>
                  <th>Improvement</th>
                  <th>Pessimistic</th>
                </tr>
              </thead>
              <tbody>
                {brief.data.options.map((o) => (
                  <tr key={o.id}>
                    <td>
                      <strong>{o.name}</strong>
                      {o.blocked?.length ? <div className="muted">{o.blocked[0]}</div> : null}
                      <div>
                        <button className="linklike" onClick={() => runProbe(o.id)}>Turn assumptions off</button>
                      </div>
                    </td>
                    <td><Pill tone={kindTone[o.kind as keyof typeof kindTone] ?? 'neutral'}>{o.kind}</Pill></td>
                    <td>{o.status}{o.rank ? ` · #${o.rank}` : ''}</td>
                    <td>{num(o.improvement)}</td>
                    <td>{num(o.pessimistic)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {probe ? <pre className="code">{probe}</pre> : null}
          </Card>
          <div className="grid-2">
            <Card title="Evidence">
              <table className="data">
                <thead>
                  <tr><th>KPI</th><th>Value</th><th>Target</th><th>Fresh</th></tr>
                </thead>
                <tbody>
                  {brief.data.evidence.filter((e) => e.severity > 0).slice(0, 8).map((e) => (
                    <tr key={e.kpi}>
                      <td>{e.name}<div className="muted">{e.owner} · {e.source}</div></td>
                      <td>{num(e.value)} {e.unit}</td>
                      <td>{num(e.target ?? 0)}</td>
                      <td>{e.fresh}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
            <Card title="Uncertainty and conflicts">
              <ul className="list">
                {brief.data.uncertainty.map((u) => <li key={u}>{u}</li>)}
                {(brief.data.conflicts ?? []).map((c) => <li key={c.summary}>{c.summary}</li>)}
              </ul>
              <p><strong>Next step.</strong> {brief.data.next_step.summary}</p>
            </Card>
          </div>
        </>
      ) : brief.error ? <ErrorNote message={brief.error} /> : null}
    </>
  );
}
