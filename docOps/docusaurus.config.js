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
  projectName: 'Goshimmer', // Usually your repo name.
  stylesheets: [
    'https://fonts.googleapis.com/css?family=Material+Icons',
    'https://iota-community.github.io/iota-wiki/assets/css/styles.f9f708da.css',//replace this URL
  ],
  themeConfig: {
    navbar: {
      title: 'Goshimmer',
      logo: {
        alt: 'IOTA',
        src: '/img/logo/Logo_Swirl_Dark.png',
      },
      items: [
//        {
//          type: 'doc',
//          docId: 'goshimmer',
//          position: 'left',
//          label: 'Documentation',
//        },
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
//        {
//          title: 'Documentation',
//          items: [
//            {
//              label: 'Welcome',
//              to: '/docs/',
//            },
//            {
//              label: 'Overview',
//              to: '/docs/overview',
//            },
//            {
//              label: 'Libraries',
//              to: '/docs/libraries/overview',
//            },
//            {
//              label: 'Specification',
//              to: '/docs/specification',
//            },
//            {
//              label: 'Contribute',
//              to: '/docs/contribute',
//            },
//          ],
//        },
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
          customCss: require.resolve('./src/css/custom.css'),
        },
      },
    ],
  ],
};
