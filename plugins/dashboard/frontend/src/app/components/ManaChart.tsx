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
        if (this.props.data.length == 0) {
            return []
        }
        return (
            <Card style={{height: '400px', paddingBottom: 80}}>
                <Card.Body>
                    <Card.Title>Mana of Node: {this.props.node}</Card.Title>
                    <Chart
                        width={'100%'}
                        height={'300px'}
                        chartType="AreaChart"
                        loader={<div>Loading Chart</div>}
                        data={[
                            [{type: 'date', id: 'Date', }, {type: 'number', label: 'Access Mana'}, {type: 'number', label: 'Consensus Mana' }],
                            ...this.props.data
                        ]}
                        options={{
                            title: '',
                            hAxis: {
                                title: 'Time',
                                titleTextStyle: { color: '#333' },
                                format:  'hh:mm:ss',
                                },
                            vAxis: { title: 'Mana', minValue: 0 },
                            legend: {position: 'top'},
                            // For the legend to fit, we make the chart area smaller
                            chartArea: { width: '80%', height: '70%' },
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
                                            chartArea: { width: '80%', height: '20%' },
                                            hAxis: { baselineColor: 'none' },
                                        },
                                    },
                                },
                                controlPosition: 'bottom',
                                controlWrapperParams: {
                                    state: {
                                        range: {
                                            // (30*60*1000) is half an hour in milliseconds
                                            start: new Date( this.props.data[this.props.data.length-1][0].getTime() - (30*60*1000)),
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