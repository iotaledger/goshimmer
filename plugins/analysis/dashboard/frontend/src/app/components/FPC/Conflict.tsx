import classNames from "classnames";
import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";
import "./Conflict.scss";
import { FPCProps } from "./FPCProps";
import { Opinion } from "../../models/opinion";

@inject("fpcStore")
@observer
export default class Conflict extends React.Component<FPCProps, unknown> {
    public componentDidMount(): void {
        this.props.fpcStore.updateCurrentConflict(this.props.match.params.id);
    }

    public render(): ReactNode {
        const { nodeConflictGrid } = this.props.fpcStore;

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
                    {!nodeConflictGrid &&
                        <div className="card">
                            <p>The node data for this conflict is no longer available.</p>
                        </div>
                    }
                    {nodeConflictGrid && Object.keys(nodeConflictGrid).map(nodeID =>
                        <div
                            key={nodeID}
                            className={classNames(
                                "card",
                                "node-details"
                            )}
                        >
                            <div className="details row middle margin-b-s">
                                <label>Node ID</label>
                                <span className="value">{nodeID}</span>
                            </div>
                            <div className="details row middle">
                                <label>Opinions</label>
                                {nodeConflictGrid[nodeID].reverse().map((opinion, idx) =>
                                    <span key={idx} className={
                                        classNames(
                                            "value",
                                            "margin-r-t",
                                            { "like": opinion === Opinion.like },
                                            { "dislike": opinion === Opinion.dislike },
                                            { "historic": idx !== 0 }
                                        )
                                    }>
                                        {opinion === Opinion.like ? "Like" : "Dislike"}
                                    </span>
                                )}
                            </div>
                        </div>
                    )}
                </div>
            </div>
        );
    }
}
