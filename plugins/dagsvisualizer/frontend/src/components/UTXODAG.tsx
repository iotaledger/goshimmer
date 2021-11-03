import * as React from 'react';
import Container from 'react-bootstrap/Container';
import {inject, observer} from "mobx-react";
import {TransactionInfo} from "components/TransactionInfo";
import UTXOStore from "stores/UTXOStore";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import Popover from "react-bootstrap/Popover";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";
import InputGroup from "react-bootstrap/InputGroup";
import FormControl from "react-bootstrap/FormControl";

interface Props {
    utxoStore?: UTXOStore;
}

@inject("utxoStore")
@observer
export class UTXODAG extends React.Component<Props, any> {
    componentDidMount() {
        this.props.utxoStore.start();
    }

    componentWillUnmount() {
        this.props.utxoStore.stop();
    }

    pauseResumeVisualizer = (e) => {
        this.props.utxoStore.pauseResume();
    }

    updateVerticesLimit = (e) => {
        this.props.utxoStore.updateVerticesLimit(e.target.value);
    }

    updateSearch = (e) => {
        this.props.utxoStore.updateSearch(e.target.value);
    }

    searchAndHighlight = (e: any) => {
        if (e.key !== 'Enter') return;
        this.props.utxoStore.searchAndHighlight();
    }

    render () {
        let { paused, maxUTXOVertices, search } = this.props.utxoStore;

        return (
            <Container>
                <h2> UTXO DAG </h2>
                <Row xs={5}>
                    <Col>
                        <InputGroup className="mb-1">
                            <OverlayTrigger
                                trigger={['hover', 'focus']} placement="right" overlay={
                                <Popover id="popover-basic">
                                    <Popover.Body>
                                        Pauses/resumes rendering the graph.
                                    </Popover.Body>
                                </Popover>}
                            >
                                <Button onClick={this.pauseResumeVisualizer} variant="outline-secondary">
                                    {paused ? "Resume Rendering" : "Pause Rendering"}
                                </Button>
                            </OverlayTrigger>
                        </InputGroup>
                    </Col>
                    <Col>
                        <InputGroup className="mb-1">
                            <InputGroup.Text id="vertices-limit">Vertices Limit</InputGroup.Text>
                            <FormControl
                                placeholder="limit"
                                value={maxUTXOVertices.toString()} onChange={this.updateVerticesLimit}
                                aria-label="vertices-limit"
                                aria-describedby="vertices-limit"
                            />
                        </InputGroup>
                    </Col>
                    <Col>
                        <InputGroup className="mb-1">
                            <InputGroup.Text id="search-vertices">
                                Search Vertex
                            </InputGroup.Text>
                            <FormControl
                                placeholder="search"
                                type="text" value={search} onChange={this.updateSearch}
                                aria-label="vertices-search" onKeyUp={this.searchAndHighlight}
                                aria-describedby="vertices-search"
                            />
                        </InputGroup>
                    </Col>                  
                </Row>
                <div className="graphFrame">
                    <TransactionInfo />
                    <div id="utxoVisualizer" />
                </div>                    
            </Container>
        );
    }
}