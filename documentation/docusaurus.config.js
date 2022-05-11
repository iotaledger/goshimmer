const path = require('path');

module.exports = {
    title: 'GoShimmer',
    url: '/',
    baseUrl: '/',
    themes: ['@docusaurus/theme-classic'],
    themeConfig: {
        navbar: {
            // Workaround to disable broken logo href on test build
            logo: {
                src: 'img/logo/goshimmer_light.png',
                href: 'https://wiki.iota.org/',
            },
        },
    },
    plugins: [
        [
            '@docusaurus/plugin-content-docs',
            {
                id: 'goshimmer',
                path: path.resolve(__dirname, 'docs'),
                routeBasePath: 'goshimmer',
                sidebarPath: path.resolve(__dirname, 'sidebars.js'),
                editUrl: 'https://github.com/iotaledger/goshimmer/edit/develop/documentation',
            }
        ],
    ],
    staticDirectories: [path.resolve(__dirname, 'static')],
};
