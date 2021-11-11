import * as React from 'react';
import Container from 'react-bootstrap/Container'
import {inject, observer} from "mobx-react";
import TangleStore from "stores/TangleStore";
import {MessageInfo} from "components/MessageInfo";
import "styles/style.css";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import Popover from "react-bootstrap/Popover";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";
import InputGroup from "react-bootstrap/InputGroup";
import FormControl from "react-bootstrap/FormControl";

interface Props {
    tangleStore?: TangleStore;
}

@inject("tangleStore")
@observer
export class TangleDAG extends React.Component<Props, any> {
    componentDidMount() {
        this.props.tangleStore.start();
    }

    componentWillUnmount() {
        this.props.tangleStore.stop();
    }

    pauseResumeVisualizer = (e) => {
        this.props.tangleStore.pauseResume();
    }

    updateVerticesLimit = (e) => {
        this.props.tangleStore.updateVerticesLimit(e.target.value);
    }

    updateSearch = (e) => {
        this.props.tangleStore.updateSearch(e.target.value);
    }

    searchAndHighlight = (e: any) => {
        if (e.key !== 'Enter') return;
        this.props.tangleStore.searchAndHighlight();
    }

    render () {
        let { paused, maxTangleVertices, search } = this.props.tangleStore;

        return (
            <Container>
                <h2> Tangle DAG </h2>
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
                                value={maxTangleVertices.toString()} onChange={this.updateVerticesLimit}
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
                    <MessageInfo />
                    <div id="tangleVisualizer" />
                </div> 
            </Container>
            
        );
    }
}