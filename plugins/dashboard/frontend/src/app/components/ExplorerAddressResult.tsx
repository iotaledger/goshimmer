import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import {Link} from 'react-router-dom';
import * as dateformat from 'dateformat';
import Alert from "react-bootstrap/Alert";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            id: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerAddressQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchAddress(this.props.match.params.id);
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.id !== this.props.match.params.id) {
            this.props.explorerStore.searchAddress(this.props.match.params.id);
        }
        return null;
    }

    render() {
        let {id} = this.props.match.params;
        let {addr, query_loading} = this.props.explorerStore;
        let msgsEle = [];
        if (addr) {
            for (let i = 0; i < addr.messages.length; i++) {
                let msg = addr.messages[i];
                msgsEle.push(
                    <ListGroup.Item key={msg.id}>
                        <small>
                            {dateformat(new Date(msg.timestamp * 1000), "dd.mm.yyyy HH:MM:ss")} {' '}
                            <Link to={`/explorer/message/${msg.id}`}>{msg.id}</Link>
                        </small>
                    </ListGroup.Item>
                );
            }
        }
        return (
            <Container>
                <h3>Address {addr !== null && <span>({addr.messages.length} Messages)</span>}</h3>
                <p>
                    {id} {' '}
                </p>
                {
                    addr !== null ?
                        <React.Fragment>
                            {
                                addr.messages !== null && addr.messages.length === 100 &&
                                <Alert variant={"warning"}>
                                    Max. 100 messages are shown.
                                </Alert>
                            }
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"}>
                                        {msgsEle}
                                    </ListGroup>
                                </Col>
                            </Row>
                        </React.Fragment>
                        :
                        <Row className={"mb-3"}>
                            <Col>
                                {query_loading && <Spinner animation="border"/>}
                            </Col>
                        </Row>
                }

            </Container>
        );
    }
}
