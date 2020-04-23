import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {Neighbor} from "app/components/Neighbor";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Neighbors extends React.Component<Props, any> {
    render() {
        let neighborsEle = [];
        this.props.nodeStore.neighbor_metrics.forEach((v, k) => {
            neighborsEle.push(<Neighbor key={k} identity={k}/>);
        });
        return (
            <Container>
                <h3>Neighbors {neighborsEle.length > 0 && <span>({neighborsEle.length})</span>}</h3>
                <p>
                    Currently connected neighbors.
                </p>
                {neighborsEle}
            </Container>
        );
    }
}
