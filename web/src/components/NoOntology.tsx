import { Card, Empty, PageHero } from './ui';

/**
 * A pack without an ontology has nothing to list on the pages that read it.
 * Say so once, with the way forward, instead of failing their requests.
 */
export default function NoOntology({
  eyebrow,
  title,
  tint,
  lede,
  what,
}: {
  eyebrow: string;
  title: string;
  tint?: 'green' | 'amber' | 'purple' | 'red';
  lede: string;
  /** Plural noun for what the page would show, e.g. "business objects". */
  what: string;
}) {
  return (
    <>
      <PageHero eyebrow={eyebrow} title={title} tint={tint} lede={lede} />
      <Card>
        <Empty
          action={
            <p className="muted small">
              Add an <code>ontology.yaml</code> to the pack, or draft one from a database with <code>zyntra ontology suggest</code>.
            </p>
          }
        >
          This pack has no ontology, so there are no {what} to show.
        </Empty>
      </Card>
    </>
  );
}
