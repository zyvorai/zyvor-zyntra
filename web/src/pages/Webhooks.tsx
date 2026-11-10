import { useState } from 'react';
import { ago, api, can } from '../api';
import { useApi } from '../hooks';
import { useWho } from '../session';
import { Card, Empty, ErrorNote, PageHero, PageSkeleton, Pill, TableScroll } from '../components/ui';

interface Destination {
  id: string;
  url: string;
  secret?: string;
  events?: string[];
  enabled: boolean;
  last_status?: number;
  last_delivery?: string;
  failures?: number;
}

export default function Webhooks() {
  const list = useApi<{ webhooks: Destination[] }>('/api/v1/webhooks', 15000);
  const who = useWho();
  const admin = can(who, 'admin');
  const [url, setUrl] = useState('');
  const [events, setEvents] = useState('pending,executed,failed');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  const create = async () => {
    setErr('');
    if (!url.trim()) return setErr('URL is required');
    setBusy(true);
    try {
      await api('/api/v1/webhooks', {
        method: 'POST',
        json: {
          url: url.trim(),
          enabled: true,
          events: events
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean),
        },
      });
      setUrl('');
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await api(`/api/v1/webhooks/${encodeURIComponent(id)}`, { method: 'DELETE' });
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  const test = async (id: string) => {
    try {
      await api(`/api/v1/webhooks/${encodeURIComponent(id)}/test`, { method: 'POST' });
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  const rows = list.data?.webhooks ?? [];

  return (
    <>
      <PageHero
        eyebrow="Automations"
        title="Webhooks"
        tint="purple"
        lede="HMAC-signed outbound events for approval transitions. Delivery is best-effort and never blocks a decision."
      />
      <ErrorNote message={list.error || err} onRetry={list.error ? list.reload : undefined} />
      {!list.data && !list.error ? <PageSkeleton stats={false} /> : null}
      <div className="page-body" hidden={!list.data && !list.error}>
        {admin ? (
          <Card title="New destination">
            <div className="manual-form">
              <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://…" aria-label="Webhook URL" />
              <input value={events} onChange={(e) => setEvents(e.target.value)} placeholder="events (comma-separated)" />
              <button className="primary" disabled={busy} onClick={create}>
                {busy ? 'Saving…' : 'Add'}
              </button>
            </div>
          </Card>
        ) : null}
        <Card title="Destinations">
          {rows.length === 0 ? (
            <Empty>No webhooks yet. Add a destination to receive signed approval events.</Empty>
          ) : (
            <TableScroll>
              <table>
                <thead>
                  <tr>
                    <th>URL</th>
                    <th>Events</th>
                    <th>Last delivery</th>
                    <th>Status</th>
                    {admin ? <th /> : null}
                  </tr>
                </thead>
                <tbody>
                  {rows.map((d) => (
                    <tr key={d.id}>
                      <td>
                        <strong className="mono small">{d.url}</strong>
                        <div className="muted mono small">{d.id}</div>
                      </td>
                      <td className="small">{(d.events ?? ['*']).join(', ')}</td>
                      <td className="small">
                        {d.last_delivery ? (
                          <>
                            {ago(d.last_delivery)}
                            {d.last_status ? ` · HTTP ${d.last_status}` : ''}
                            {d.failures ? ` · ${d.failures} fail(s)` : ''}
                          </>
                        ) : (
                          <span className="muted">never</span>
                        )}
                      </td>
                      <td>
                        <Pill tone={d.enabled ? 'ok' : 'neutral'}>{d.enabled ? 'enabled' : 'paused'}</Pill>
                      </td>
                      {admin ? (
                        <td className="row-actions">
                          <button onClick={() => test(d.id)}>Test</button>
                          <button className="danger" onClick={() => remove(d.id)}>
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
