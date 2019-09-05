package dashboard

var tpsTemplate = `
<!DOCTYPE html>
<html>

<head>
    <script src="https://code.jquery.com/jquery-3.1.1.min.js"></script>
    <script src="https://code.highcharts.com/stock/highstock.js"></script>
    <script src="https://code.highcharts.com/stock/modules/exporting.js"></script>
    <script src="https://code.highcharts.com/stock/modules/export-data.js"></script>
</head>

<body>
    <div id="node-status" style="min-width:310px">
        <h3 id="node-id"></h3>
        <h3 id="node-neighbor"></h3>
        <h3 id="node-knownpeer"></h3>
        <h3 id="node-uptime"></h3>
    </div>
    <div id="container" style="height: 400px; min-width: 310px"></div>
    <script>
        Highcharts.createElement('link', {
            href: 'https://fonts.googleapis.com/css?family=Unica+One',
            rel: 'stylesheet',
            type: 'text/css'
        }, null, document.getElementsByTagName('head')[0]);

        Highcharts.theme = {
            colors: ['#2b908f', '#90ee7e', '#f45b5b', '#7798BF', '#aaeeee', '#ff0066',
                '#eeaaee', '#55BF3B', '#DF5353', '#7798BF', '#aaeeee'],
            chart: {
                backgroundColor: {
                    linearGradient: { x1: 0, y1: 0, x2: 1, y2: 1 },
                    stops: [
                        [0, '#2a2a2b'],
                        [1, '#3e3e40']
                    ]
                },
                // style: {
                //     fontFamily: 'sans-serif'
                // },
                plotBorderColor: '#606063'
            },
            title: {
                style: {
                    color: '#E0E0E3',
                    // textTransform: 'uppercase',
                    fontSize: '20px'
                }
            },
            subtitle: {
                style: {
                    color: '#E0E0E3',
                    // textTransform: 'uppercase'
                }
            },
            xAxis: {
                gridLineColor: '#707073',
                labels: {
                    style: {
                        color: '#E0E0E3'
                    }
                },
                lineColor: '#707073',
                minorGridLineColor: '#505053',
                tickColor: '#707073',
                title: {
                    style: {
                        color: '#A0A0A3'

                    }
                }
            },
            yAxis: {
                gridLineColor: '#707073',
                labels: {
                    style: {
                        color: '#E0E0E3'
                    }
                },
                lineColor: '#707073',
                minorGridLineColor: '#505053',
                tickColor: '#707073',
                tickWidth: 1,
                title: {
                    style: {
                        color: '#A0A0A3'
                    }
                }
            },
            tooltip: {
                backgroundColor: 'rgba(0, 0, 0, 0.85)',
                style: {
                    color: '#F0F0F0'
                }
            },
            plotOptions: {
                series: {
                    dataLabels: {
                        color: '#B0B0B3'
                    },
                    marker: {
                        lineColor: '#333'
                    }
                },
                boxplot: {
                    fillColor: '#505053'
                },
                candlestick: {
                    lineColor: 'white'
                },
                errorbar: {
                    color: 'white'
                }
            },
            legend: {
                itemStyle: {
                    color: '#E0E0E3'
                },
                itemHoverStyle: {
                    color: '#FFF'
                },
                itemHiddenStyle: {
                    color: '#606063'
                }
            },
            credits: {
                style: {
                    color: '#666'
                }
            },
            labels: {
                style: {
                    color: '#707073'
                }
            },

            drilldown: {
                activeAxisLabelStyle: {
                    color: '#F0F0F3'
                },
                activeDataLabelStyle: {
                    color: '#F0F0F3'
                }
            },

            navigation: {
                buttonOptions: {
                    symbolStroke: '#DDDDDD',
                    theme: {
                        fill: '#505053'
                    }
                }
            },

            // scroll charts
            rangeSelector: {
                buttonTheme: {
                    fill: '#505053',
                    stroke: '#000000',
                    style: {
                        color: '#CCC'
                    },
                    states: {
                        hover: {
                            fill: '#707073',
                            stroke: '#000000',
                            style: {
                                color: 'white'
                            }
                        },
                        select: {
                            fill: '#000003',
                            stroke: '#000000',
                            style: {
                                color: 'white'
                            }
                        }
                    }
                },
                inputBoxBorderColor: '#505053',
                inputStyle: {
                    backgroundColor: '#333',
                    color: 'silver'
                },
                labelStyle: {
                    color: 'silver'
                }
            },

            navigator: {
                handles: {
                    backgroundColor: '#666',
                    borderColor: '#AAA'
                },
                outlineColor: '#CCC',
                maskFill: 'rgba(255,255,255,0.1)',
                series: {
                    color: '#7798BF',
                    lineColor: '#A6C7ED'
                },
                xAxis: {
                    gridLineColor: '#505053'
                }
            },

            scrollbar: {
                barBackgroundColor: '#808083',
                barBorderColor: '#808083',
                buttonArrowColor: '#CCC',
                buttonBackgroundColor: '#606063',
                buttonBorderColor: '#606063',
                rifleColor: '#FFF',
                trackBackgroundColor: '#404043',
                trackBorderColor: '#404043'
            },

            // special colors for some of the
            legendBackgroundColor: 'rgba(0, 0, 0, 0.5)',
            background2: '#505053',
            dataLabelsColor: '#B0B0B3',
            textColor: '#C0C0C0',
            contrastTextColor: '#F0F0F3',
            maskColor: 'rgba(255,255,255,0.3)'
        };

        // Apply the theme
        Highcharts.setOptions(Highcharts.theme);

        // Start Here
        var time = Date.now() - 3000;
        var start_draw = time + 5000;
        const update_rate = 5;
        var counter = 0;
        data = [[time - 5000, 0]];
        var tzoffset = new Date().getTimezoneOffset();
        var chart = Highcharts.stockChart('container', {
            rangeSelector: {
                selected: 1
            },

            title: {
                text: 'Transactions per second'
            },

            time: {
                timezoneOffset: tzoffset
            },

            rangeSelector: {
                buttons: [
                    {
                        type: 'minute',
                        count: 5,
                        text: '5m'
                    }, {
                        type: 'minute',
                        count: 15,
                        text: '15m'
                    }, {
                        type: 'minute',
                        count: 30,
                        text: '30m'
                    }, {
                        type: 'hour',
                        count: 60,
                        text: '1h'
                    }],
                inputEnabled: false
            },

            series: [{
                name: 'Transactions per second',
                data: data,
                type: 'areaspline',
                threshold: null,
                tooltip: {
                    valueDecimals: 0
                },
                fillColor: {
                    linearGradient: {
                        x1: 0,
                        y1: 0,
                        x2: 0,
                        y2: 1
                    },
                    stops: [
                        [0, Highcharts.getOptions().colors[0]],
                        [1, Highcharts.Color(Highcharts.getOptions().colors[0]).setOpacity(0).get('rgba')]
                    ]
                }
            }]
        });

        const dataStr = '{{.Data}}'
        const parsedData = JSON.parse(dataStr.replace(/ /g, ','));
        time = Date.now() - 1000 * (parsedData.length + 1);
        for (let i = 0; i < parsedData.length; i++) {
            chart.series[0].addPoint([time += 1000, parseInt(parsedData[i], 10)], false);
        }
		chart.redraw();
        console.log(parsedData);

        const conn = new WebSocket("ws://{{.Host}}/ws");
        conn.binaryType = 'arraybuffer';
        conn.onopen = evt => {
            console.log("WebSocket is open now.");
            console.log("WebSocket done.");
        }
        conn.onclose = () => {
            // console.log('Connection closed');
        }
        conn.onmessage = evt => {
            const data = evt.data;
            if(isJSON(data)) {
                console.log('status updated');
                var res = JSON.parse(data);
                $("#node-id").html("Node ID: " + res.Id);
                $("#node-neighbor").html(res.Neighbor);
                $("#node-knownpeer").html(res.KnownPeer);
                $("#node-uptime").html(res.Uptime);
            } else {
                console.log('metric updated');
                const dv = new DataView(data);
                // var value = dv.getUint32(4, true) << 32 | dv.getUint32(0, true);
                const value = dv.getUint32(0, true);
                chart.series[0].addPoint([time += 1000, value], true); //((counter += 1) % update_rate == 4));
                console.log(value);
                console.log(dv);
            }
        }

        function isJSON(str) {
            try {
                JSON.parse(str);
            } catch(e) {
                return false;
            }
            return true;
        }
    </script>
</body>

</html>
`
