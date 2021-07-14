const lightCodeTheme = require('prism-react-renderer/themes/github');
const darkCodeTheme = require('prism-react-renderer/themes/dracula');

/** @type {import('@docusaurus/types').DocusaurusConfig} */
module.exports = {
  title: 'Goshimmer',
  tagline: 'Official Goshimmer Software',
  url: 'https://goshimmer.docs.iota.org/',
  baseUrl: '/goshimmer/',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'throw',
  favicon: '/img/logo/favicon.ico',
  organizationName: 'iotaledger', // Usually your GitHub org/user name.
  projectName: 'GoShimmer', // Usually your repo name.
  stylesheets: [
    'https://fonts.googleapis.com/css?family=Material+Icons',
  ],
  themeConfig: {
    colorMode: {
          defaultMode: "dark",
          },
    navbar: {
      title: 'goShimmer',
      logo: {
        alt: 'IOTA',
        src: 'img/logo/Logo_Swirl_Dark.png',
      },
      items: [
        {
          type: 'doc',
          docId: 'welcome',
          position: 'left',
          label: 'Documentation',
        },
        {
          href: 'https://github.com/iotaledger/Goshimmer',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
        footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Welcome',
              to: '/docs/welcome',
            },
            {
              label: 'FAQ',
              to: '/docs/faq',
            },
            {
              label: 'Tutorials',
              to: '/docs/tutorials/set-up-a-goshimmer-node',
            },
            {
              label: 'Implementation Design',
              to: '/docs/implementation_design/event_driven_model',
            },
            {
              label: 'Protocol Specification',
              to: '/docs/protocol_specification',
            },
            {
              label: 'API',
              to: '/docs/apis/api',
            },
            {
              label: 'Tooling',
              to: '/docs/tooling',
            },
            {
              label: 'Team Resources',
              to: '/docs/teamresources/release',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/iotaledger/Goshimmer',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} IOTA Foundation, Built with Docusaurus.`,
    },
    prism: {
        additionalLanguages: ['rust'],
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
    },
  },
  presets: [
    [
      '@docusaurus/preset-classic',
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          // Please change this to your repo.
          editUrl:
            'https://github.com/iotaledger/Goshimmer/tree/main/docs',
        },
        theme: {
          customCss: require.resolve('./src/css/iota.css'),
        },
      },
    ],
  ],
};
