import { shortenedIDCharCount } from "app/stores/AutopeeringStore";
import { inject, observer } from "mobx-react";
import * as React from 'react';
import { Link } from 'react-router-dom';
import "./FPCItem.scss";
import { FPCItemProps } from './FPCItemProps';

@inject("fpcStore")
@observer
export default class FPCItem extends React.Component<FPCItemProps, any> {
    render() {
        const total = Object.keys(this.props.nodeOpinions).length;
        return (
            <Link
                to={`/consensus/conflict/${this.props.conflictID}`}
                className="fpc-item"
            >
                <div
                    className="percentage"
                    style={{
                        width: `${Math.floor((this.props.likes / total) * 200)}px`
                    }}
                />
                <div className="label">
                    {`${this.props.conflictID.substr(0, shortenedIDCharCount)}: ${this.props.likes} / ${total}`}
                </div>
            </Link>
        );
    }
}
