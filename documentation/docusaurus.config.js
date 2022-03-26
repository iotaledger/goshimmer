const path = require('path');

module.exports = {
    title: 'GoShimmer',
    url: '/',
    baseUrl: '/',
    themes: ['@docusaurus/theme-classic'],
    plugins: [
        [
            '@docusaurus/plugin-content-docs',
            {
                id: 'goshimmer',
                path: path.resolve(__dirname, './docs'),
                routeBasePath: 'goshimmer',
                sidebarPath: path.resolve(__dirname, './sidebars.js'),
                editUrl: 'https://github.com/iotaledger/goshimmer/edit/develop/',
            }
        ],
    ],
    staticDirectories: [path.resolve(__dirname, './static')],
};
