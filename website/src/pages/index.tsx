import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import CodeBlock from '@theme/CodeBlock';
import FeatureHighlights from '@site/src/components/FeatureHighlights';
import ScreenshotStrip from '@site/src/components/ScreenshotStrip';
import Reveal from '@site/src/components/Reveal';

import styles from './index.module.css';

const DEMO = 'https://zyvor.dev/schedule?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_hero';
const POC = 'https://zyvor.dev/poc?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_hero';

function HomepageHeader() {
  const hero = useBaseUrl('/img/shot-overview.jpg');
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <div className={styles.heroGrid}>
          <div>
            <p className={styles.eyebrow}>Decision intelligence for operations</p>
            <Heading as="h1" className="hero__title">
              Five dashboards are red.
              <br />
              Know the next best action, and why.
            </Heading>
            <p className="hero__subtitle">
              Zyntra keeps a live graph of the KPIs you are judged on, shows which
              ones miss target and by how much, simulates every candidate action
              and ranks them. Nothing runs until a named person approves it, and
              every result is checked against the prediction.
            </p>
            <div className={styles.buttons}>
              <Link className="button button--secondary button--lg" to={DEMO}>
                Book a demo
              </Link>
              <Link className="button button--outline button--lg button--secondary" to={POC}>
                30-day PoC
              </Link>
              <Link
                className="button button--outline button--lg button--secondary"
                to="/docs/getting-started/quickstart">
                Quickstart
              </Link>
            </div>
            <p className={styles.heroWall}>
              Read-only sources · Explainable what-if · Human approval on every
              change · Dry-run by default · Signed decision records · One Go binary
            </p>
          </div>
          <div className={styles.heroMedia}>
            <img
              src={hero}
              alt="Zyntra overview showing three KPIs off target, a grounded digest and the ranked recommended actions"
            />
            <p className={styles.heroMediaCaption}>
              What is off target, and what to do first. On one screen.
            </p>
          </div>
        </div>
      </div>
    </header>
  );
}

const OUTCOMES: [string, string][] = [
  [
    'One ranked answer.',
    'Gaps sorted worst first by criticality, and every candidate action ranked on its pessimistic case, with the reason.',
  ],
  [
    'See it before it runs.',
    'A what-if through your KPI graph shows the predicted change to every KPI, with a range and the full path behind each number.',
  ],
  [
    'A person decides.',
    'Roles, quorum and change windows. The prediction is re-checked right before anything runs, and dry-run is the default.',
  ],
  [
    'Proof you can hand over.',
    'Every decision is a signed, hash-chained record that verifies offline, with predicted against actual per KPI.',
  ],
];

function Outcomes() {
  return (
    <section className={styles.problem}>
      <div className="container">
        <Reveal>
          <div className={styles.qGrid}>
            {OUTCOMES.map(([kicker, body]) => (
              <div key={kicker} className={styles.qCard}>
                <strong>{kicker}</strong>
                <span>{body}</span>
              </div>
            ))}
          </div>
          <p className={clsx('text--center', styles.subNote)}>
            Runs on the data you already have: CSV exports, Prometheus, SQL, REST
            APIs, Kubernetes and webhooks.
          </p>
        </Reveal>
      </div>
    </section>
  );
}

const LOOP: [string, string][] = [
  ['Sense', 'Read-only sources refresh the KPI graph'],
  ['Find the gap', 'KPIs off target, weighted by criticality'],
  ['Simulate', 'Actions propagate through declared edges, with ranges'],
  ['Rank', 'Scored on the pessimistic case; breaches blocked'],
  ['Approve', 'A named person or quorum, inside the change window'],
  ['Revalidate', 'Re-simulated on fresh data right before running'],
  ['Act', 'Kubernetes, webhook or file, dry-run by default'],
  ['Verify', 'Predicted against actual; rollback proposal on regression'],
];

function DecisionLoop() {
  return (
    <section className={styles.questions}>
      <div className="container">
        <Reveal>
          <Heading as="h2" className={clsx(styles.sectionHeading, 'text--center')}>
            Every change approved, re-checked and proven
          </Heading>
          <p className={clsx('text--center', styles.subNote)}>
            A person always stands between a recommendation and a change.
          </p>
          <ol className={styles.loop}>
            {LOOP.map(([step, detail]) => (
              <li key={step}>
                <strong>{step}</strong>
                <span>{detail}</span>
              </li>
            ))}
          </ol>
        </Reveal>
      </div>
    </section>
  );
}

const USE_CASES: [string, string][] = [
  [
    'End the war room',
    'When several KPIs go red at once, everyone starts from the same ranked plan and the reasoning behind it.',
  ],
  [
    'Run GPU capacity',
    'Weigh Gryvia priority, GPU sharing and job suspend against queue and latency, with Netra network signals in the graph.',
  ],
  [
    'Start from a CSV',
    'Point a retail, manufacturing, logistics, payments or imaging operations pack at an export and get a ranked plan.',
  ],
  [
    'Investigate a KPI change',
    'Find sustained shifts and changes that moved before them, with downloadable evidence. Leads, not root causes.',
  ],
  [
    'Answer from your runbooks',
    'Ask questions over your SOPs and policies and get cited excerpts, filtered by who is allowed to read them.',
  ],
  [
    'Let scripts propose, not approve',
    'Service tokens let agents and scripts propose a change. A named person still approves every one.',
  ],
];

function UseCases() {
  return (
    <section className={styles.trust}>
      <div className="container">
        <Reveal>
          <Heading as="h2" className={clsx(styles.sectionHeading, 'text--center')}>
            From a red dashboard to a decision you can defend
          </Heading>
          <div className={styles.useGrid}>
            {USE_CASES.map(([title, body]) => (
              <div key={title} className={styles.qCard}>
                <strong>{title}</strong>
                <span>{body}</span>
              </div>
            ))}
          </div>
        </Reveal>
      </div>
    </section>
  );
}

const COMPARE: [string, string, string][] = [
  [
    'The question it answers',
    'What the metrics look like now and over time',
    'Which action closes the gap, and what it does to every other KPI',
  ],
  ['The model', 'Panels and queries', 'A KPI graph with targets, owners and declared cause-and-effect edges'],
  ['What-if', 'Not part of a dashboard', 'Candidate actions simulated with ranges, then ranked'],
  [
    'Taking action',
    'Outside the tool, in runbooks and other systems',
    'An approval lane with roles, quorum and change windows, dry-run by default',
  ],
  ['After the change', 'Someone watches the panels', 'Predicted against actual per KPI, and a rollback proposal on regression'],
  ['Evidence', 'Dashboards and annotations', 'A hash-chained audit trail and signed decision records'],
];

function Compare() {
  return (
    <section className={styles.enterprise}>
      <div className="container">
        <Reveal>
          <Heading as="h2" className={clsx(styles.sectionHeading, 'text--center')}>
            Your dashboards show the problem. Zyntra decides the fix.
          </Heading>
          <div className={styles.tableWrap}>
            <table className={styles.compare}>
              <thead>
                <tr>
                  <th />
                  <th>Typical dashboard and alerting tool</th>
                  <th className={styles.lead}>Zyntra</th>
                </tr>
              </thead>
              <tbody>
                {COMPARE.map(([label, typical, ours]) => (
                  <tr key={label}>
                    <td>{label}</td>
                    <td>{typical}</td>
                    <td className={styles.lead}>{ours}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className={clsx('text--center', styles.subNote)}>
            Dashboards are the right tool for visualising and alerting across many
            sources. Zyntra reads the same sources and turns what they show into a
            ranked, approved, verified decision.
          </p>
        </Reveal>
      </div>
    </section>
  );
}

function Quickstart() {
  return (
    <section className={styles.quickstart}>
      <div className="container">
        <Reveal className={styles.quickGrid}>
          <div>
            <Heading as="h2" className={styles.sectionHeading}>
              A ranked plan in minutes
            </Heading>
            <p>
              Build one Go binary and get a ranked plan from the shop pack's CSV
              fixtures. No cluster needed. Then point a pack at your own exports,
              Prometheus, SQL, REST, Kubernetes or webhooks.
            </p>
            <p>
              Sign in as <code>admin</code> / <code>Admin@321</code> until you set{' '}
              <code>ZYNTRA_ADMIN_PASSWORD</code>.
            </p>
            <Link to="/docs/getting-started/quickstart">Read the quickstart →</Link>
          </div>
          <div>
            <CodeBlock language="bash">
              {`git clone https://github.com/zyvorai/zyntra.git && cd zyntra
make build
./bin/zyntra plan -f packs/shop      # ranked plan from CSV fixtures
ZYNTRA_API_KEY=dev ./bin/zyntra serve -f packs/shop   # console on :8080

# or the signed container
docker run --rm -p 8080:8080 -e ZYNTRA_API_KEY=dev ghcr.io/zyvorai/zyntra:edge`}
            </CodeBlock>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function TrustBand() {
  return (
    <section className={styles.trust}>
      <div className="container">
        <Reveal className={styles.trustGrid}>
          <div>
            <Heading as="h3" className={styles.sectionHeading}>
              Built to be trusted, and honest about its limits
            </Heading>
            <p>
              No model decides: the AI layer explains and forecasts, and never
              picks, ranks or runs an action. Sources only read. The simulator is
              a deterministic model over the weights you declare, not a
              measurement; <code>zyntra calibrate</code> backtests it against real
              outcomes and only suggests corrections.
            </p>
            <Link to="/docs/security">Read the security model →</Link>
          </div>
          <div className={styles.trustBadges}>
            <img src="https://github.com/zyvorai/zyntra/actions/workflows/ci.yml/badge.svg" alt="CI status" />
            <img
              src="https://img.shields.io/badge/License-Zyvor%20Production%20v1.0-ff5a15.svg"
              alt="Zyvor Production License v1.0"
            />
            <img src="https://img.shields.io/badge/Go-one%20binary-00ADD8?logo=go&logoColor=white" alt="One Go binary" />
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function FinalCTA() {
  return (
    <section className={styles.enterprise}>
      <div className="container text--center">
        <Reveal>
          <Heading as="h2" className={styles.sectionHeading}>
            Know the next best action before the next war room
          </Heading>
          <p className={styles.enterpriseCopy}>
            Free for evaluation, development, labs and other non-production use
            under the Zyvor Production License. Production needs a commercial
            license, sized by managed clusters and KPI graphs, with unlimited users
            and approvers.
          </p>
          <div className={clsx(styles.buttons, styles.centerButtons)}>
            <Link className="button button--primary button--lg" to={DEMO}>
              Book a demo
            </Link>
            <Link className="button button--outline button--primary button--lg" to={POC}>
              Start a 30-day PoC
            </Link>
            <Link className="button button--outline button--primary button--lg" to="https://github.com/zyvorai/zyntra">
              Star on GitHub
            </Link>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Zyntra — decision intelligence for operations"
      description="A live KPI graph, explainable what-if simulation, a ranked plan, human approval on every change and signed, verified outcomes. Free for non-production.">
      <HomepageHeader />
      <main>
        <Outcomes />
        <Reveal>
          <FeatureHighlights />
        </Reveal>
        <DecisionLoop />
        <UseCases />
        <Compare />
        <Reveal>
          <ScreenshotStrip />
        </Reveal>
        <Quickstart />
        <TrustBand />
        <FinalCTA />
      </main>
    </Layout>
  );
}
