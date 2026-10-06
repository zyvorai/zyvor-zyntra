import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type Shot = {
  src: string;
  alt: string;
};

const SHOTS: Shot[] = [
  {src: '/img/shot-workflows.jpg', alt: 'Workflows — orders exposed to a failing KPI'},
  {src: '/img/shot-model.jpg', alt: 'Model — targets, owners and declared edges'},
  {src: '/img/shot-overview.jpg', alt: 'Overview — what is off target and what to do first'},
];

export default function ScreenshotStrip(): ReactNode {
  return (
    <section className={styles.strip}>
      <div className="container">
        <Heading as="h2" className="text--center">
          One console for operators, owners and approvers
        </Heading>
        <p className="text--center">
          Real pages from the lab model and the manufacturing pack. No mock-ups.{' '}
          <Link to="/gallery">See the full tour →</Link>
        </p>
        <div className={styles.grid}>
          {SHOTS.map((shot) => (
            <ShotImage key={shot.src} shot={shot} />
          ))}
        </div>
      </div>
    </section>
  );
}

function ShotImage({shot}: {shot: Shot}) {
  const src = useBaseUrl(shot.src);
  return (
    <Link to="/gallery" className={styles.frame}>
      <img src={src} alt={shot.alt} loading="lazy" />
    </Link>
  );
}
