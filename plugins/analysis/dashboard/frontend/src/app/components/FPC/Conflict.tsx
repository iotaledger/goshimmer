import classNames from "classnames";
import { inject, observer } from "mobx-react";
import * as React from 'react';
import "./Conflict.scss";
import { FPCProps } from './FPCProps';

@inject("fpcStore")
@observer
export default class Conflict extends React.Component<FPCProps, any> {
    componentDidMount() {
        this.props.fpcStore.updateCurrentConflict(this.props.match.params.id);
    }

    render() {
        let { nodeConflictGrid } = this.props.fpcStore;

        return (
            <div className="conflict">
                <div className="header margin-b-m">
                    <h2>Conflict Detail</h2>
                </div>
                <div className="card margin-b-m">
                    <div className="details row middle">
                        <label>Conflict ID</label>
                        <span className="value">{this.props.match.params.id}</span>
                    </div>
                </div>
                <div className="node-grid">
                    {!nodeConflictGrid && (
                        <div className="card">
                            <p>The node data for this conflict is no longer available.</p>
                        </div>
                    )}
                    {nodeConflictGrid && nodeConflictGrid.map(nodeDetails => (
                        <div
                            key={nodeDetails.nodeID}
                            className={classNames(
                                "card",
                                "node-details",
                                { like: nodeDetails.opinion === 1 },
                                { dislike: nodeDetails.opinion === 2 }
                            )}
                        >
                            <div className="details row middle">
                                <label>Node ID</label>
                                <span className="value">{nodeDetails.nodeID}</span>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
        );
    }
}
