import * as React from 'react';
import Card from "react-bootstrap/Card";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {Line} from "react-chartjs-2";
import {defaultChartOptions} from "app/misc/Chart";

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
                    return Math.abs(value);
                },
                fontSize: 10,
                maxTicksLimit: 4,
                beginAtZero: true,
            },
        }],
    },
    tooltips: {
        callbacks: {
            label: function (tooltipItem, data) {
                let label = data.datasets[tooltipItem.datasetIndex].label;
                return `${label} ${Math.abs(tooltipItem.value)}`;
            }
        }
    }
}, defaultChartOptions);

@inject("nodeStore")
@observer
export default class StoreChart extends React.Component<Props, any> {
    render() {
        const infoStyle = {
            display: "flex",
            flexDirection: "column"
        };
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Component Counter Blocks Per Second</Card.Title>
                    <div style={infoStyle as React.CSSProperties}>
                        <small>
                            BPS: {this.props.nodeStore.last_component_counter_metric.store}.
                        </small>
                        <small>
                            Rate Setter - Estimate: {this.props.nodeStore.last_rate_setter_metric.estimate}
                        </small>
                    </div>

                    <Line height={50} data={this.props.nodeStore.componentSeries} options={lineChartOptions}/>
                </Card.Body>
            </Card>
        );
    }
}
