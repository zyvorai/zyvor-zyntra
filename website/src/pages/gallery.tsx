import type {ReactNode} from 'react';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './gallery.module.css';

type Shot = {
  src: string;
  caption: string;
};

const TOUR: Shot[] = [
  {src: '/img/login.png', caption: 'Sign in'},
  {src: '/img/shot-plan.jpg', caption: 'Plan: every action ranked, with the reasoning'},
  {src: '/img/shot-simulate.jpg', caption: 'Simulate: see the change before it happens'},
  {src: '/img/shot-approvals.jpg', caption: 'Approvals: nothing runs until a person says so'},
  {src: '/img/shot-objects.jpg', caption: 'Objects: what depends on what, with provenance'},
  {src: '/img/shot-scenarios.jpg', caption: 'Scenarios: compare plans side by side'},
  {src: '/img/shot-workflows.jpg', caption: 'Workflows: the orders a failing KPI puts at risk'},
  {src: '/img/shot-signals.jpg', caption: 'Signals: live sources, trend and target status'},
  {src: '/img/shot-model.jpg', caption: 'Model: targets, owners and declared edges'},
];

function ShotCard({shot}: {shot: Shot}) {
  const src = useBaseUrl(shot.src);
  return (
    <figure className={styles.shot}>
      <a href={src} target="_blank" rel="noreferrer">
        <img src={src} alt={shot.caption} loading="lazy" />
      </a>
      <figcaption>{shot.caption}</figcaption>
    </figure>
  );
}

export default function Gallery(): ReactNode {
  const hero = useBaseUrl('/img/shot-overview.jpg');
  return (
    <Layout
      title="Product tour"
      description="A walkthrough of the Zyntra console: real pages from the lab model and the manufacturing pack.">
      <header className={styles.header}>
        <div className="container">
          <Heading as="h1">Product tour</Heading>
          <p>
            Every screenshot below is a real Zyntra console page from the lab
            model and the manufacturing pack — not a mockup.
          </p>
        </div>
      </header>
      <main className="container">
        <div className={styles.demo}>
          <img src={hero} alt="Zyntra overview" />
          <p className={styles.caption}>
            Overview: KPIs off target, a grounded digest and the ranked
            recommended actions. What to do first, on one screen.
          </p>
        </div>
        <div className={styles.grid}>
          {TOUR.map((shot) => (
            <ShotCard key={shot.src} shot={shot} />
          ))}
        </div>
      </main>
    </Layout>
  );
}
