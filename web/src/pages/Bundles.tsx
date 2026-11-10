import { useState } from 'react';
import { api, can } from '../api';
import { useWho } from '../session';
import { Card, ErrorNote, PageHero } from '../components/ui';

export default function Bundles() {
  const who = useWho();
  const admin = can(who, 'admin');
  const [exportText, setExportText] = useState('');
  const [importText, setImportText] = useState('');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  const runExport = async () => {
    setErr('');
    setMsg('');
    setBusy(true);
    try {
      const b = await api<unknown>('/api/v1/bundles/export', { method: 'POST' });
      setExportText(JSON.stringify(b, null, 2));
      setMsg('Exported schedules, webhook destinations (secrets omitted) and built-in templates.');
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const runImport = async () => {
    setErr('');
    setMsg('');
    setBusy(true);
    try {
      let parsed: unknown;
      try {
        parsed = JSON.parse(importText);
      } catch {
        setErr('Import text must be valid JSON');
        return;
      }
      const result = await api<{ imported: Record<string, number> }>('/api/v1/bundles/import', {
        method: 'POST',
        json: parsed,
      });
      setMsg(`Imported: ${JSON.stringify(result.imported)}`);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  if (!admin) {
    return (
      <>
        <PageHero eyebrow="Automations" title="Bundles" tint="green" lede="Export and import pack automation subsets." />
        <ErrorNote message="Bundles require the admin role." />
      </>
    );
  }

  return (
    <>
      <PageHero
        eyebrow="Automations"
        title="Bundles"
        tint="green"
        lede="Versioned export and import of schedules and webhook destinations. Secrets are never exported."
      />
      <ErrorNote message={err} />
      {msg ? <p className="muted">{msg}</p> : null}
      <div className="page-body">
        <Card title="Export">
          <button className="primary" disabled={busy} onClick={runExport}>
            {busy ? 'Working…' : 'Export now'}
          </button>
          {exportText ? <pre className="mono small bundle-json">{exportText}</pre> : null}
        </Card>
        <Card title="Import">
          <textarea
            rows={12}
            className="mono"
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
            placeholder='{"version":1,"pack":"…","items":{…}}'
            aria-label="Bundle JSON"
          />
          <button className="primary" disabled={busy || !importText.trim()} onClick={runImport}>
            Import
          </button>
        </Card>
      </div>
    </>
  );
}
