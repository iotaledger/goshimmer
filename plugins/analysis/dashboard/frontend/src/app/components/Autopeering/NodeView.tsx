import classNames from "classnames";
import { shortenedIDCharCount } from "../../stores/AutopeeringStore";
import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";
import "./NodeView.scss";
import { AutopeeringProps } from "./AutopeeringProps";

@inject("autopeeringStore")
@inject("manaStore")
@observer
export class NodeView extends React.Component<AutopeeringProps, unknown> {
    public render(): ReactNode {
        return !this.props.autopeeringStore.selectedNode ? null :
            <div className="card node-view">
                <div className="card--header">
                    <h3>
                        Node {this.props.autopeeringStore.selectedNode}
                    </h3>
                    {
                        this.props.autopeeringStore.manaColoringActive &&
                        <h3>
                            Active Cons. Mana: {this.props.manaStore.getActiveCMana(this.props.autopeeringStore.selectedNode)}
                        </h3>
                    }

                </div>
                <div className="row margin-t-s">
                    <div className="col">
                        <label className="margin-b-t">
                            Incoming
                            <span className="badge">{
                                this.props.autopeeringStore.selectedNodeInNeighbors ?
                                    this.props.autopeeringStore.selectedNodeInNeighbors.size.toString() : 0}</span>
                        </label>
                        <div className="node-view--list">
                            {this.props.autopeeringStore.inNeighborList.map(nodeId =>
                                <button
                                    key={nodeId}
                                    onClick={() => this.props.autopeeringStore.handleNodeSelection(nodeId)}
                                    className={classNames(
                                        {
                                            "preview-incoming": nodeId === this.props.autopeeringStore.previewNode
                                        }
                                    )}
                                >
                                    {nodeId.substr(0, shortenedIDCharCount)}
                                </button>
                            )}
                        </div>
                    </div>

                    <div className="col">
                        <label className="margin-b-t">
                            Outgoing
                            <span className="badge">{this.props.autopeeringStore.selectedNodeOutNeighbors ?
                                this.props.autopeeringStore.selectedNodeOutNeighbors.size.toString() : 0}</span>
                        </label>
                        <div className="node-view--list">
                            {this.props.autopeeringStore.outNeighborList.map(nodeId =>
                                <button
                                    key={nodeId}
                                    onClick={() => this.props.autopeeringStore.handleNodeSelection(nodeId)}
                                    className={classNames(
                                        {
                                            "preview-outgoing": nodeId === this.props.autopeeringStore.previewNode
                                        }
                                    )}
                                >
                                    {nodeId.substr(0, shortenedIDCharCount)}
                                </button>
                            )}
                        </div>
                    </div>
                </div>
            </div>;
    }
}
