import * as React from 'react';
import Container from 'react-bootstrap/Container';
import Row from 'react-bootstrap/Row';
import TangleDAG from 'components/TangleDAG';
import UTXODAG from 'components/UTXODAG';
import BranchDAG from 'components/BranchDAG';
import GlobalSettings from 'components/GlobalSettings';
import { connectWebSocket } from 'utils/WS';

export class Root extends React.Component {
    connect = () => {
        connectWebSocket(
            '/ws',
            () => {
                console.log('connection opened');
            },
            this.reconnect,
            () => {
                console.log('connection error');
            }
        );
    };

    reconnect = () => {
        setTimeout(() => {
            this.connect();
        }, 1000);
    };

    componentDidMount(): void {
        this.connect();
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
        );
    }
}
