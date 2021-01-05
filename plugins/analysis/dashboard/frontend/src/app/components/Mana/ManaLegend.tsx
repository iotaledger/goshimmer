import {observer} from "mobx-react";
import * as React from "react";
import {Col, Row} from "react-bootstrap";

interface Props {
    min: string;
    mid: string;
    max: string;
}


@observer
export default class ManaLegend extends React.Component<Props, any> {
    render() {
        return (
            <div className="mana-legend">
                <Row>
                    <Col>
                        <div className="bar"/>
                    </Col>
                </Row>
                <Row>
                    <Col style={{textAlign: "left"}}>
                        <small>{this.props.min}</small>
                    </Col>
                    <Col style={{textAlign: "center"}}>
                        <small>{this.props.mid}</small>
                    </Col>
                    <Col style={{textAlign: "right"}}>
                        <small>{this.props.max}</small>
                    </Col>
                </Row>
            </div>
        );
    }
}