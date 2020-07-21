import { shortenedIDCharCount } from "../../stores/AutopeeringStore";
import classNames from "classnames";
import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";
import "./Autopeering.scss";
import { AutopeeringProps } from "./AutopeeringProps";
import { NodeView } from "./NodeView";
import Dropdown from 'react-bootstrap/Dropdown';
import DropdownButton from 'react-bootstrap/DropdownButton';

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
                        <DropdownButton id="dropdown-basic-button" title={this.props.autopeeringStore.selectedNetworkVersion === null ? "Select network version": "Network: " + this.props.autopeeringStore.selectedNetworkVersion}>
                            <Dropdown.Item
                                as="button"
                                key={"clear"}
                                onClick={() => this.props.autopeeringStore.handleVersionSelection("")}>
                                clear
                            </Dropdown.Item>
                            {this.props.autopeeringStore.networkVersionList.map(version => (
                                <Dropdown.Item
                                    as="button"
                                    key={version}
                                    onClick={() => this.props.autopeeringStore.handleVersionSelection(version)}
                                >
                                    {version}
                                </Dropdown.Item>
                            ))}

                        </DropdownButton>
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
                <div className="visualizer" id="visualizer" />
            </div>
        );
    }
}
