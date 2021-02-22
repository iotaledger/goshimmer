import {observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import {Col, Container, Row} from "react-bootstrap";
import {displayManaUnit} from "../../../../../../../dashboard/frontend/src/app/utils/";


interface Props {
    title: string;
    value: number;
}

@observer
export default class ManaGauge extends React.Component<Props, any> {
    render() {
        const currentValue = this.props.value;
        return (
            <Card>
                <Card.Body>
                    <Card.Title style={{marginBottom: '0rem'}}>
                        <Container fluid style={{padding: '0rem'}}>
                            <Row>
                                <Col>
                                    {this.props.title}
                                </Col>
                                <Col>
                                    <b>{displayManaUnit(currentValue)}</b>
                                </Col>
                            </Row>
                        </Container>
                    </Card.Title>
                </Card.Body>
            </Card>
        );
    }
}

