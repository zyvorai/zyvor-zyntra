import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Zyntra',
  tagline: 'Five dashboards are red. Know the next best action, and why.',
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: 'https://zyvorai.github.io',
  baseUrl: '/zyntra/',

  organizationName: 'zyvorai',
  projectName: 'zyntra',

  onBrokenLinks: 'throw',

  markdown: {
    // Docs are generated from ../docs/*.md, which use CommonMark (`<channel>`, `{id}` in prose).
    format: 'detect',
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/zyvorai/zyntra/tree/main/website/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      defaultMode: 'dark',
      respectPrefersColorScheme: false,
    },
    navbar: {
      title: 'Zyntra',
      logo: {
        alt: 'Zyntra',
        src: 'img/favicon.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/gallery',
          label: 'Product tour',
          position: 'left',
        },
        {
          href: 'https://zyvor.dev/schedule?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_nav',
          label: 'Book a demo',
          position: 'right',
        },
        {
          href: 'https://github.com/zyvorai/zyntra',
          label: 'GitHub',
          position: 'right',
        },
        {
          href: 'https://zyvor.dev/zyntra',
          label: 'Zyvor',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {label: 'Quickstart', to: '/docs/getting-started/quickstart'},
            {label: 'Deploy', to: '/docs/getting-started/deploy'},
            {label: 'Decision loop', to: '/docs/core-concepts/decision-loop'},
            {label: 'Reference', to: '/docs/reference'},
            {label: 'Security', to: '/docs/security'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: 'https://github.com/zyvorai/zyntra'},
            {label: 'Releases', href: 'https://github.com/zyvorai/zyntra/releases'},
            {label: 'Contributing', href: 'https://github.com/zyvorai/zyntra/blob/main/CONTRIBUTING.md'},
            {
              label: 'License (Zyvor Production v1.0)',
              href: 'https://github.com/zyvorai/zyntra/blob/main/LICENSE',
            },
          ],
        },
        {
          title: 'Zyvor',
          items: [
            {label: 'zyvor.dev', href: 'https://zyvor.dev/zyntra'},
            {
              label: 'Book a demo',
              href: 'https://zyvor.dev/schedule?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_footer',
            },
            {
              label: '30-day PoC',
              href: 'https://zyvor.dev/poc?utm_source=github-pages&utm_medium=zyntra&utm_campaign=docs_footer',
            },
            {label: 'sales@zyvor.dev', href: 'mailto:sales@zyvor.dev'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Zyvor AI Labs Private Limited. Zyvor Production License v1.0.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'json', 'yaml', 'http'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
