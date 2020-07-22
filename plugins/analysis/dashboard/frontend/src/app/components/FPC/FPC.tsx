import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";

import { CSSTransition, TransitionGroup } from "react-transition-group";
import "./FPC.scss";
import FPCItem from "./FPCItem";
import { FPCProps } from "./FPCProps";

@inject("fpcStore")
@observer
export default class FPC extends React.Component<FPCProps, unknown> {
    public componentDidMount(): void {
        this.props.fpcStore.start();
    }

    public componentWillUnmount(): void {
        this.props.fpcStore.stop();
    }

    public render(): ReactNode {
        const { conflictGrid } = this.props.fpcStore;
        return (
            <div className="fpc">
                <div className="header margin-b-m">
                    <h2>Conflicts Overview</h2>
                </div>
                <div className="conflict-grid">
                    {conflictGrid.length === 0 && 
                        <p>There are no conflicts to show.</p>
                    }
                    <TransitionGroup>
                        {conflictGrid.map(conflict => 
                            <CSSTransition
                                className="fpc-item"
                                key={conflict.conflictID}
                                timeout={300}
                            >
                                <FPCItem
                                    {...conflict}
                                />
                            </CSSTransition>
                        )}
                    </TransitionGroup>
                </div>
            </div>
        );
    }
}
