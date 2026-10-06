import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  description: ReactNode;
  shot: string;
  alt: string;
  to: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Start from a ranked plan',
    description:
      'Gaps weighted by criticality; single actions and pairs ranked by what they close, minus risk and stale inputs. A plan that only wins if every edge holds is flagged and ranked lower.',
    shot: '/img/shot-plan.jpg',
    alt: 'Plan page with every action ranked and the reason',
    to: '/docs/core-concepts/decision-loop',
  },
  {
    title: 'See it before it runs',
    description:
      'Simulate one action or several through the dependency graph: before, after, range and target for each KPI, with the arithmetic behind every number.',
    shot: '/img/shot-simulate.jpg',
    alt: 'Simulate page with before, after, range and target per KPI',
    to: '/docs/core-concepts/decision-loop',
  },
  {
    title: 'The business behind the number',
    description:
      'A business ontology links orders, services and machines to the KPIs that measure them, with the source and time behind every fact, and permission-aware reads.',
    shot: '/img/shot-objects.jpg',
    alt: 'Objects page showing a failing service and the orders that depend on it',
    to: '/docs/core-concepts/ontology',
  },
  {
    title: 'Compare plans side by side',
    description:
      'Save a what-if with its assumptions, re-run it against current data and compare it with the alternatives. Running a scenario never changes anything.',
    shot: '/img/shot-scenarios.jpg',
    alt: 'Scenarios page comparing three plans side by side',
    to: '/docs/core-concepts/ontology',
  },
  {
    title: 'Live, read-only signals',
    description:
      'A dead source shows up as stale or down instead of quietly wrong. KPI watches open an incident only when a breach is sustained, with measured recovery.',
    shot: '/img/shot-signals.jpg',
    alt: 'Signals page listing live sources with trend and target status',
    to: '/docs/core-concepts/watches',
  },
  {
    title: 'Nothing runs until a person says so',
    description:
      'The approval lane shows the baseline against the prediction and the exact change that will run, with roles, quorum and change windows.',
    shot: '/img/shot-approvals.jpg',
    alt: 'Approvals page showing the prediction and the exact change',
    to: '/docs/core-concepts/decision-loop',
  },
];

function Feature({title, description, shot, alt, to}: FeatureItem) {
  const src = useBaseUrl(shot);
  return (
    <div className="col col--6">
      <Link to={to} className={styles.card}>
        <img src={src} alt={alt} loading="lazy" className={styles.shot} />
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </Link>
    </div>
  );
}

export default function FeatureHighlights(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <Heading as="h2" className={styles.heading}>
          Everything between a missed target and a verified fix. In one console.
        </Heading>
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
