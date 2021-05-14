import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    txId: string
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerTransactionAttachments extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getTransactionAttachments(this.props.txId);
    }
    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let { txAttachments } = this.props.explorerStore;
        return (
            <div style={{marginTop: "20px", marginBottom: "20px"}}>
                <h4>Attachments</h4>
                {txAttachments && txAttachments.messageIDs && <Row className={"mb-3"}>
                   <Col>
                       <ListGroup>
                           {txAttachments.messageIDs.map((msgId, i) => {
                               return <ListGroup.Item key={i}><a href={`/explorer/message/${msgId}`}>{msgId}</a></ListGroup.Item>
                           })}
                       </ListGroup>
                   </Col>
                </Row>}
            </div>
        )
    }
}