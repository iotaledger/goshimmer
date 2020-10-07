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
                            series: {
                                0: {color: '#213e3b'},
                                1: {color: '#41aea9'}
                            },
                            legend: {position: 'top'},
                            // For the legend to fit, we make the chart area smaller
                            chartArea: { width: '80%', height: '70%' },
                            // lineWidth: 25
                            explorer: { actions: ["dragToPan", "rightClickToReset"],
                                keepInBounds: true
                            },
                        }}
                        // For tests
                        rootProps={{ 'data-testid': '1' }}
                    />
                </Card.Body>
            </Card>
        );
    }
}