import { shortenedIDCharCount } from "../../stores/AutopeeringStore";
import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";

import { Link } from "react-router-dom";
import "./FPCItem.scss";
import { FPCItemProps } from "./FPCItemProps";

@inject("fpcStore")
@observer
export default class FPCItem extends React.Component<FPCItemProps, unknown> {
    public render(): ReactNode {
        const total = Object.keys(this.props.nodeOpinions).length;
        const likes = this.props.likes ?? 0;

        return (
            <Link
                to={`/consensus/conflict/${this.props.conflictID}`}
                className="fpc-item"
            >
                <div
                    className="percentage"
                    style={{
                        width: `${Math.floor(likes / total * 200)}px`
                    }}
                />
                <div className="label">
                    {`${this.props.conflictID.substr(0, shortenedIDCharCount)}: ${likes} / ${total}`}
                </div>
            </Link>
        );
    }
}
