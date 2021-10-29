import * as React from 'react';
import Container from 'react-bootstrap/Container'
import {inject, observer} from "mobx-react";
import BranchStore from "stores/BranchStore";
import {BranchInfo} from "components/BranchInfo";
import "styles/style.css";

interface Props {
    branchStore?: BranchStore;
}

@inject("branchStore")
@observer
export class BranchDAG extends React.Component<Props, any> {
    componentDidMount() {
        this.props.branchStore.start();
    }

    componentWillUnmount() {
        this.props.branchStore.stop();
    }

    render () {
        return (
            <Container>
                <h2> Branch DAG </h2>
                <div className="graphFrame">
                    <BranchInfo />
                    <div id="branchVisualizer" />
                </div> 
            </Container>
            
        );
    }

}