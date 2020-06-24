import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {FaucetAddressInput} from "app/components/FaucetAddressInput";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Faucet extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>GoShimmer Faucet</h3>
                <Row className={"mb-3"}>
                    <Col>
                        <p>
                            Get tokens from the GoShimmer faucet!
                        </p>
                    </Col>
                </Row>
                <FaucetAddressInput/>
            </Container>
        );
    }
}
