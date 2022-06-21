import * as React from 'react';
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
export default class DeficitChart extends React.Component<Props, any> {
    render() {
        const infoStyle = {
            display: "flex",
            flexDirection: "column"
        };
        return (
            <div className={"row"}>
                <div className={"col"}>
                    <div style={infoStyle as React.CSSProperties}>
                        <small>
                            Current deficit: {this.props.nodeStore.status.scheduler.deficit}.
                        </small>
                    </div>

                    <Line height={50} data={this.props.nodeStore.deficitSeries} options={lineChartOptions}/>
                </div>
            </div>
        );
    }
}
