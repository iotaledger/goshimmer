import {observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import {Chart} from "react-google-charts"

interface Props {
    data;
    title;
}

@observer
export default class ManaHistogram extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>{this.props.title}</Card.Title>
                    <Chart
                        width={'100%'}
                        height={'400px'}
                        chartType="Histogram"
                        loader={<div>Loading Chart</div>}
                        data={[
                            ['NodeID', {type: 'number', label: 'log10(Mana)' }],
                            ...this.props.data
                        ]}
                        options={{
                            title: 'Mana Histogram',
                            hAxis: { title: 'log10(Mana) of Nodes ', titleTextStyle: { color: '#333' } },
                            vAxis: { title: 'Number of Nodes in Bucket', titleTextStyle: { color: '#333' } },
                            // By default, Google Charts will choose the bucket size automatically,
                            // using a well-known algorithm for histograms.
                            // Histogram chart provides two options: histogram.bucketSize, which overrides the
                            // algorithm and hardcodes the bucket size; and histogram.lastBucketPercentile. The
                            // second option needs more explanation: it changes the computation of bucket sizes
                            // to ignore the values that are higher or lower than the remaining values by the
                            // percentage you specify. The values are still included in the histogram, but do
                            // not affect how they're bucketed. This is useful when you don't want outliers to
                            // land in their own buckets; they will be grouped with the first or last buckets instead.
                            //histogram: { lastBucketPercentile: 5 },
                            legend: {position: 'none'},
                            colors: ['#41aea9']
                        }}
                        // For tests
                        rootProps={{ 'data-testid': '1' }}
                    />
                </Card.Body>
            </Card>
        );
    }
}