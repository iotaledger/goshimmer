import {UnlockBlock as unlockBlockJSON} from "app/misc/Payload";
import * as React from "react";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Badge from "react-bootstrap/Badge";
import ListGroup from "react-bootstrap/ListGroup";
import {resolveSignatureType} from "app/utils/unlock_block";

interface UnlockProps {
    block: unlockBlockJSON;
    index: number;
    key: number;
}

export class UnlockBlock extends React.Component<UnlockProps, any> {
    render() {
        let block = this.props.block;
        return (
            <Row className={"mb-3"}>
                <Col>
                    Index: <Badge variant={"primary"}>{this.props.index}</Badge>
                    <ListGroup>
                        <ListGroup.Item>Type: {block.type}</ListGroup.Item>
                        {
                            block.referencedIndex && <ListGroup.Item>Referenced Index: {block.referencedIndex}</ListGroup.Item>
                        }
                        {
                            block.signatureType && <ListGroup.Item>Signature Type: {resolveSignatureType(block.signatureType)}</ListGroup.Item>
                        }
                        {
                            block.signature && <ListGroup.Item>Signature: {block.signature}</ListGroup.Item>
                        }
                        {
                            block.publicKey && <ListGroup.Item>Public Key: {block.publicKey}</ListGroup.Item>
                        }
                    </ListGroup>
                </Col>
            </Row>
        );
    }
}