import * as React from 'react';
import Container from 'react-bootstrap/Container';
import Row from "react-bootstrap/Row";
import {inject, observer} from "mobx-react";
import TangleStore from 'stores/TangleStore';
import {TangleDAG} from 'components/TangleDAG';
import {UTXODAG} from 'components/UTXODAG';
import {BranchDAG} from 'components/BranchDAG';
import { GlobalSettings } from 'components/GlobalSettings';

interface Props {
    tangleStore?: TangleStore;
}

@inject("tangleStore")
@observer
export class Root extends React.Component<Props, any> {
    componentDidMount(): void {
        this.props.tangleStore.connect();
    }

    render() {
        return (
            <Container>
                <Row>
                    <GlobalSettings />
                </Row>
                <Row>
                    <TangleDAG />
                </Row>
                <Row>
                    <UTXODAG />
                </Row>
                <Row>
                    <BranchDAG />
                </Row>
            </Container>
        )
    }
}