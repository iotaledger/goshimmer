import { shortenedIDCharCount } from "../../stores/AutopeeringStore";
import classNames from "classnames";
import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";
import "./Autopeering.scss";
import { AutopeeringProps } from "./AutopeeringProps";
import { NodeView } from "./NodeView";
import BootstrapSwitchButton from 'bootstrap-switch-button-react'
import ManaLegend from "../Mana/ManaLegend";

@inject("autopeeringStore")
@observer
export default class Autopeering extends React.Component<AutopeeringProps, unknown> {
    public componentDidMount(): void {
        this.props.autopeeringStore.start();
    }

    public componentWillUnmount(): void {
        this.props.autopeeringStore.stop();
    }

    public render(): ReactNode {
        const { nodeListView, search } = this.props.autopeeringStore;
        return (
            <div className="auto-peering">
                <div className="header margin-b-m">
                    <h2>Autopeering Visualizer</h2>
                    <div className="row">
                        <select
                            onChange={(e) => this.props.autopeeringStore.handleVersionSelection(e.target.value)}
                            value={this.props.autopeeringStore.selectedNetworkVersion}
                        >
                            {this.props.autopeeringStore.versions.size === 0 && (
                                <option>No data for any network</option>
                            )}
                            {this.props.autopeeringStore.networkVersionList.map(version => (
                                <option value={version} key={version}>Network {version}</option>
                            ))}
                        </select>
                        <div className="badge neighbors">
                            Average number of neighbors: {this.props.autopeeringStore.AvgNumNeighbors}
                        </div>
                        <div className="badge online">
                            Nodes online: {this.props.autopeeringStore.NodesOnline}
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
                            {nodeListView.length === 0 && search.length > 0 &&
                                <p>There are no nodes to view with the current search parameters.</p>
                            }
                            {nodeListView.map((nodeId) =>
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
                            )}
                        </div>
                    </div>
                    <div className="node-view-container">
                        {!this.props.autopeeringStore.selectedNode &&
                            <div className="card">
                                <p className="margin-t-t">Select a node to inspect its details.</p>
                            </div>
                        }
                        {this.props.autopeeringStore.selectedNode &&
                            <NodeView {...this.props} />
                        }
                    </div>
                </div>

                <div className="visualizer" id="visualizer" >
                    <div className="controls">
                        Active Consensus Mana <BootstrapSwitchButton
                            size="xs"
                            onstyle="dark"
                            checked={this.props.autopeeringStore.manaColoringActive}
                            onlabel='On'
                            offlabel='Off'
                            onChange={(checked: boolean) => {
                                this.props.autopeeringStore.handleManaColoringChange(checked);
                            }}
                        />
                    </div>
                    <div>
                        {
                            this.props.autopeeringStore.manaColoringActive &&
                            <ManaLegend min={'0 m'} mid={"1e8 m (100Mm)"} max={"1e15 (1Pm)"} />
                        }
                    </div>
                </div>
            </div>
        );
    }
}
