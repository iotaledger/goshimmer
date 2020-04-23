import * as React from 'react';
import Card from "react-bootstrap/Card";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {Line} from "react-chartjs-2";
import {defaultChartOptions} from "app/misc/Chart";
import * as prettysize from 'prettysize';

interface Props {
    nodeStore?: NodeStore;
}

const lineChartOptions = Object.assign({
    scales: {
        xAxes: [{
            ticks: {
                autoSkip: true,
                maxTicksLimit: 8,
                fontSize: 8,
                minRotation: 0,
                maxRotation: 0,
            },
            showXLabels: 10,
            gridLines: {
                display: false
            }
        }],
        yAxes: [{
            gridLines: {
                display: false
            },
            ticks: {
                callback: function (value, index, values) {
                    return prettysize(Math.abs(value));
                },
                maxTicksLimit: 3,
                fontSize: 10,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${prettysize(Math.abs(tooltipItem.value))}`;
            }
        }
    }
}, defaultChartOptions);

@inject("nodeStore")
@observer
export default class MemChart extends React.Component<Props, any> {
    render() {
        let mem = this.props.nodeStore.status.mem;
        return (
            <Card>
                <Card.Body>
                    <Card.Title>
                        Memory Usage{' '}
                        {prettysize(mem.heap_inuse + (mem.heap_idle - mem.heap_released) + mem.m_span_inuse + mem.m_cache_inuse+mem.stack_sys)}
                    </Card.Title>
                    <small>
                        GC Cycles: {mem.num_gc} (Last Cycle: {mem.last_pause_gc / 1000000}ms) - {' '}
                        Heap: {' '}
                        [Obj: {mem.heap_objects}, In-Use: {prettysize(mem.heap_inuse)},
                        Retained: {prettysize(mem.heap_idle - mem.heap_released)}]
                    </small>
                    <Line height={50}
                          data={this.props.nodeStore.memSeries} options={lineChartOptions}
                    />
                </Card.Body>
            </Card>
        );
    }
}
