import * as React from 'react';
import Container from 'react-bootstrap/Container'
import {inject, observer} from "mobx-react";
import TangleStore from "stores/TangleStore";
import "styles/style.css";

interface Props {
    tangleStore?: TangleStore;
}

@inject("tangleStore")
@observer
export class TangleDAG extends React.Component<Props, any> {
    componentDidMount() {
        this.props.tangleStore.start();
    }

    componentWillUnmount() {
        this.props.tangleStore.unregisterHandlers();
    }

    render () {
        return (
            <Container>
                <h2> Tangle DAG </h2>
                <div id="tangleVisualizer" />
            </Container>
            
        );
    }
}