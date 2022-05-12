const path = require('path');

module.exports = {
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
