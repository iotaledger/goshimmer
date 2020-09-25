import {observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import {Chart} from "react-google-charts"

interface Props {
    node;
    data;
}

@observer
export default class ManaChart extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Mana of Node: {this.props.node}</Card.Title>
                    <Chart
                        width={'auto'}
                        height={'auto'}
                        chartType="AreaChart"
                        loader={<div>Loading Chart</div>}
                        data={[
                            [{type: 'date', id: 'Date', }, {type: 'number', label: 'Access Mana'}, {type: 'number', label: 'Consensus Mana' }],
                            ...this.props.data
                        ]}
                        options={{
                            title: '',
                            hAxis: { title: 'Time', titleTextStyle: { color: '#333' } },
                            vAxis: { minValue: 0 },
                            legend: {position: 'top'},
                            // For the legend to fit, we make the chart area smaller
                            //chartArea: { width: '50%', height: '70%' },
                            // lineWidth: 25
                        }}
                        // For tests
                        rootProps={{ 'data-testid': '1' }}
                        controls={[
                            {
                                controlType: 'ChartRangeFilter',
                                options: {
                                    filterColumnIndex: 0,
                                    ui: {
                                        chartType: 'LineChart',
                                        chartOptions: {
                                            chartArea: { width: '90%', height: '50%' },
                                            hAxis: { baselineColor: 'none' },
                                        },
                                    },
                                },
                                controlPosition: 'bottom',
                                controlWrapperParams: {
                                    state: {
                                        range: {
                                            start: new Date( this.props.data[this.props.data.length-1][0].getTime() - (5*60*1000)),
                                            end: this.props.data[this.props.data.length-1][0] },
                                    },
                                },
                            },
                        ]}
                    />
                </Card.Body>
            </Card>
        );
    }
}