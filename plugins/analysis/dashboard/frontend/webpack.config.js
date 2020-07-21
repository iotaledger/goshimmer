let webpack = require('webpack');
let path = require('path');

// variables
let isProduction =
    process.argv.indexOf('-p') >= 0 || process.env.NODE_ENV === 'production';
let sourcePath = path.join(__dirname, './src');
let outPath = path.join(__dirname, './build');

// plugins
let HtmlWebpackPlugin = require('html-webpack-plugin');
let { CleanWebpackPlugin } = require('clean-webpack-plugin');

module.exports = {
    context: sourcePath,
    entry: {
        app: './main.tsx'
    },
    output: {
        path: outPath,
        publicPath: isProduction ? "/app" : "http://127.0.0.1:9090/",
        filename: isProduction ? '[contenthash].js' : '[hash].js',
        chunkFilename: isProduction ? '[name].[contenthash].js' : '[name].[hash].js'
    },
    target: 'web',
    resolve: {
        extensions: ['.js', '.ts', '.tsx'],
        // Fix webpack's default behavior to not load packages with jsnext:main module
        // (jsnext:main directs not usually distributable es6 format, but es6 sources)
        mainFields: ['module', 'browser', 'main'],
        alias: {
            app: path.resolve(__dirname, 'src/app/')
        }
    },
    module: {
        rules: [
            // .ts, .tsx
            {
                test: /\.tsx?$/,
                use: [
                    !isProduction && {
                        loader: 'babel-loader',
                        options: {
                            plugins: ['react-hot-loader/babel']
                        }
                    },
                    {
                        loader: 'ts-loader'
                    }
                ].filter(Boolean)
            },
            // scss
            {
                test: /\.s[ac]ss$/i,
                use: [
                    // Creates `style` nodes from JS strings
                    'style-loader',
                    // Translates CSS into CommonJS
                    'css-loader',
                    // Compiles Sass to CSS
                    'sass-loader',
                ],
            },
            // static assets
            { test: /\.html$/, use: 'html-loader' },
            { test: /\.(a?png|svg)$/, use: 'url-loader?limit=10000' },
            {
                test: /\.(jpe?g|gif|bmp|mp3|mp4|ogg|wav|eot|ttf|woff|woff2)$/,
                use: 'file-loader'
            }
        ]
    },
    optimization: {
        splitChunks: {
            name: true,
            cacheGroups: {
                commons: {
                    chunks: 'initial',
                    minChunks: 2
                },
                vendors: {
                    test: /[\\/]node_modules[\\/]/,
                    chunks: 'all',
                    priority: -10,
                    filename: isProduction ? 'vendor.[contenthash].js' : 'vendor.[hash].js'
                }
            }
        },
        runtimeChunk: true
    },
    plugins: [
        new webpack.EnvironmentPlugin({
            NODE_ENV: 'development', // use 'development' unless process.env.NODE_ENV is defined
            DEBUG: false
        }),
        new CleanWebpackPlugin(),
        new HtmlWebpackPlugin({
            template: 'assets/index.html'
        })
    ],
    devServer: {
        contentBase: sourcePath,
        hot: true,
        inline: true,
        disableHostCheck: true,
        historyApiFallback: {
            disableDotRule: true
        },
        stats: 'minimal',
        clientLogLevel: 'warning',
        headers: {
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, PATCH, OPTIONS",
            "Access-Control-Allow-Headers": "X-Requested-With, content-type, Authorization"
        }
    },
    // https://webpack.js.org/configuration/devtool/
    devtool: isProduction ? 'hidden-source-map' : 'cheap-module-eval-source-map',
    node: {
        // workaround for webpack-dev-server issue
        // https://github.com/webpack/webpack-dev-server/issues/60#issuecomment-103411179
        fs: 'empty',
        net: 'empty'
    }
};
