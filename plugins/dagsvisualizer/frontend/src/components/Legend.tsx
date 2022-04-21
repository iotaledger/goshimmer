import * as React from 'react';
import { COLOR } from '../styles/tangleStyles';
import { BRANCH, UTXO } from '../styles/cytoscapeStyles';

export class TangleLegend extends React.Component<any, any> {
    render() {
        const nodeLabels = [
            'MSG Confirmed',
            'MSG Pending',
            'MSG with TX Confirmed',
            'MSG with TX Pending',
            'Tip',
            'Unknown'
        ];
        const nodeColors = [
            COLOR.MESSAGE_CONFIRMED,
            COLOR.MESSAGE_PENDING,
            COLOR.TRANSACTION_CONFIRMED,
            COLOR.TRANSACTION_PENDING,
            COLOR.TIP,
            COLOR.NODE_UNKNOWN
        ];
        const linkLabels = [
            'Strong',
            'Weak',
            'Shallow Liked',
            'Shallow Disliked'
        ];
        const linksColors = [
            COLOR.LINK_STRONG,
            COLOR.LINK_WEAK,
            COLOR.LINK_SHALLOW_LIKED,
            COLOR.LINK_SHALLOW_DISLIKED
        ];
        const linkTypes = ['solid', 'dashed', 'dotted', 'dotted'];

        const legendItemsNodes = [];
        const legendItemsLinks = [];

        for (const i in nodeLabels) {
            legendItemsNodes.push(
                <div className={'legend-item'}>
                    <div
                        className="legend-color"
                        style={{
                            backgroundColor: nodeColors[i]
                        }}
                    />
                    <div className="legend-label">{nodeLabels[i]}</div>
                </div>
            );
        }
        legendItemsLinks.push(
            <div className={'legend-item'}>
                <div
                    className="legend-color"
                    style={{
                        backgroundColor: COLOR.MESSAGE_CONFIRMED
                    }}
                >
                    <div className={'legend-marker'} />
                </div>
                <div className="legend-label">Marker</div>
            </div>
        );
        for (const i in linkLabels) {
            legendItemsLinks.push(
                <div className={'legend-item'}>
                    <div
                        className="link-type"
                        style={{
                            borderBottom: `${linkTypes[i]} 2px ${linksColors[i]}`
                        }}
                    />
                    <div className="legend-label">{linkLabels[i]}</div>
                </div>
            );
        }
        return (
            <>
                <div className="legend">{legendItemsNodes}</div>
                <div className="legend">{legendItemsLinks}</div>
            </>
        );
    }
}

export class UTXOLegend extends React.Component<any, any> {
    render() {
        const nodeLabels = ['TX Confirmed', 'TX Pending'];
        const nodeColors = [UTXO.COLOR_CONFIRMED, UTXO.PARENT_COLOR];

        const legendItemsNodes = [];

        for (const i in nodeLabels) {
            legendItemsNodes.push(
                <div className={'legend-item'}>
                    <div
                        className="legend-color"
                        style={{
                            backgroundColor: nodeColors[i]
                        }}
                    />
                    <div className="legend-label">{nodeLabels[i]}</div>
                </div>
            );
        }
        return (
            <>
                <div className="legend">{legendItemsNodes}</div>
            </>
        );
    }
}

export class BranchLegend extends React.Component<any, any> {
    render() {
        const nodeLabels = [
            'Conflict branch confirmed',
            'Conflict branch pending/rejected',
            'Master branch'
        ];
        const nodeColors = [
            BRANCH.COLOR_CONFIRMED,
            BRANCH.COLOR,
            BRANCH.MASTER_COLOR
        ];

        const legendItemsNodes = [];

        for (const i in nodeLabels) {
            legendItemsNodes.push(
                <div className={'legend-item'}>
                    <div
                        className="legend-color"
                        style={{
                            backgroundColor: nodeColors[i]
                        }}
                    />
                    <div className="legend-label">{nodeLabels[i]}</div>
                </div>
            );
        }
        return (
            <>
                <div className="legend">{legendItemsNodes}</div>
            </>
        );
    }
}
