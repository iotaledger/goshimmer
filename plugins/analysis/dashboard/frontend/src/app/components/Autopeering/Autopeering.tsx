import { shortenedIDCharCount } from "app/stores/AutopeeringStore";
import classNames from "classnames";
import { inject, observer } from "mobx-react";
import * as React from 'react';
import "./Autopeering.scss";
import { AutopeeringProps } from './AutopeeringProps';
import { NodeView } from "./NodeView";

@inject("autopeeringStore")
@observer
export default class Autopeering extends React.Component<AutopeeringProps, any> {

    componentDidMount(): void {
        this.props.autopeeringStore.start();
    }

    componentWillUnmount(): void {
        this.props.autopeeringStore.stop();
    }

    render() {
        let { nodeListView, search } = this.props.autopeeringStore
        return (
            <div className="auto-peering">
                <div className="header margin-b-m">
                    <h2>Autopeering Visualizer</h2>
                    <div className="row">
                        <div className="badge neighbors">
                            Average number of neighbors: {
                                this.props.autopeeringStore.nodes && this.props.autopeeringStore.nodes.size > 0 ?
                                    (2 * this.props.autopeeringStore.connections.size / this.props.autopeeringStore.nodes.size).toPrecision(2).toString()
                                    : 0
                            }
                        </div>
                        <div className="badge online">
                            Nodes online: {this.props.autopeeringStore.nodes.size.toString()}
                        </div>
                    </div>
                </div>
                <div className="nodes-container margin-b-s">
                    <div className="card nodes">
                        <div className="row middle margin-b-s">
                            <label>
                                Search Node
                            </label>
                            <input
                                placeholder="Enter a node id"
                                type="text"
                                value={search}
                                onChange={(e) => this.props.autopeeringStore.updateSearch(e.target.value)}
                            />
                        </div>
                        <div className="node-list">
                            {nodeListView.length === 0 && search.length > 0 && (
                                <p>There are no nodes to view with the current search parameters.</p>
                            )}
                            {nodeListView.map((nodeId) => (
                                <button
                                    key={nodeId}
                                    onClick={() => this.props.autopeeringStore.handleNodeSelection(nodeId)}
                                    className={classNames(
                                        {
                                            selected: this.props.autopeeringStore.selectedNode === nodeId
                                        }
                                    )}
                                >
                                    {nodeId.substr(0, shortenedIDCharCount)}
                                </button>
                            ))}
                        </div>
                    </div>
                    <div className="node-view-container">
                        {!this.props.autopeeringStore.selectedNode && (
                            <div className="card">
                                <p className="margin-t-t">Select a node to inspect its details.</p>
                            </div>
                        )}
                        {this.props.autopeeringStore.selectedNode && (
                            <NodeView {...this.props} />
                        )}
                    </div>
                </div>
                <div className="visualizer" id="visualizer" />
            </div>
        );
    }
}
