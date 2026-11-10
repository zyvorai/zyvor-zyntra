import { useState } from 'react';
import { ago, api, can } from '../api';
import { useApi } from '../hooks';
import { useWho } from '../session';
import { Card, Empty, ErrorNote, PageHero, PageSkeleton, Pill, TableScroll } from '../components/ui';

interface Schedule {
  id: string;
  name?: string;
  pack?: string;
  every_minutes: number;
  enabled: boolean;
  last_run?: string;
  owner?: string;
  action?: string;
}

export default function Schedules() {
  const list = useApi<{ schedules: Schedule[] }>('/api/v1/schedules', 15000);
  const who = useWho();
  const admin = can(who, 'admin');
  const [name, setName] = useState('');
  const [every, setEvery] = useState('60');
  const [owner, setOwner] = useState('');
  const [action, setAction] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  const create = async () => {
    setErr('');
    const mins = Number(every);
    if (!Number.isFinite(mins) || mins < 1) return setErr('Interval must be at least 1 minute');
    setBusy(true);
    try {
      await api('/api/v1/schedules', {
        method: 'POST',
        json: { name, every_minutes: mins, enabled: true, owner: owner || undefined, action: action || undefined },
      });
      setName('');
      setOwner('');
      setAction('');
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const toggle = async (sc: Schedule) => {
    try {
      await api(`/api/v1/schedules/${encodeURIComponent(sc.id)}`, {
        method: 'PUT',
        json: { ...sc, enabled: !sc.enabled },
      });
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  const remove = async (id: string) => {
    try {
      await api(`/api/v1/schedules/${encodeURIComponent(id)}`, { method: 'DELETE' });
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  const rows = list.data?.schedules ?? [];

  return (
    <>
      <PageHero
        eyebrow="Automations"
        title="Schedules"
        tint="amber"
        lede="Recurring plan runs that open pending proposals. A schedule never executes an action — a person still approves."
      />
      <ErrorNote message={list.error || err} onRetry={list.error ? list.reload : undefined} />
      {!list.data && !list.error ? <PageSkeleton stats={false} /> : null}
      <div className="page-body" hidden={!list.data && !list.error}>
        {admin ? (
          <Card title="New schedule">
            <div className="manual-form">
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Name" aria-label="Schedule name" />
              <input type="number" min={1} value={every} onChange={(e) => setEvery(e.target.value)} placeholder="Every minutes" aria-label="Interval minutes" />
              <input value={owner} onChange={(e) => setOwner(e.target.value)} placeholder="Owner filter (optional)" />
              <input value={action} onChange={(e) => setAction(e.target.value)} placeholder="Action id (optional)" />
              <button className="primary" disabled={busy} onClick={create}>
                {busy ? 'Saving…' : 'Add'}
              </button>
            </div>
          </Card>
        ) : null}
        <Card title="Schedules">
          {rows.length === 0 ? (
            <Empty>No schedules yet. Add an interval schedule to open proposals on a cadence.</Empty>
          ) : (
            <TableScroll>
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Every</th>
                    <th>Owner / action</th>
                    <th>Last run</th>
                    <th>Status</th>
                    {admin ? <th /> : null}
                  </tr>
                </thead>
                <tbody>
                  {rows.map((sc) => (
                    <tr key={sc.id}>
                      <td>
                        <strong>{sc.name || sc.id}</strong>
                        <div className="muted mono small">{sc.id}</div>
                      </td>
                      <td>{sc.every_minutes} min</td>
                      <td className="small">
                        {sc.owner || '—'} / {sc.action || 'top plan'}
                      </td>
                      <td className="small">{sc.last_run ? ago(sc.last_run) : <span className="muted">never</span>}</td>
                      <td>
                        <Pill tone={sc.enabled ? 'ok' : 'neutral'}>{sc.enabled ? 'enabled' : 'paused'}</Pill>
                      </td>
                      {admin ? (
                        <td className="row-actions">
                          <button onClick={() => toggle(sc)}>{sc.enabled ? 'Pause' : 'Enable'}</button>
                          <button className="danger" onClick={() => remove(sc.id)}>
                            Delete
                          </button>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </TableScroll>
          )}
        </Card>
      </div>
    </>
  );
}
