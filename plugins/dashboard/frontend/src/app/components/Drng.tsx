import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {DrngLiveFeed} from "app/components/DrngLiveFeed";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Drng extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>dRNG Beacons</h3>
                <DrngLiveFeed/>
            </Container>
        );
    }
}
