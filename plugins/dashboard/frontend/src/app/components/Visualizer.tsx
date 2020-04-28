import * as React from 'react';
import {KeyboardEvent} from 'react';
import Container from "react-bootstrap/Container";
import {inject, observer} from "mobx-react";
import {Link} from 'react-router-dom';
import VisualizerStore from "app/stores/VisualizerStore";
import NodeStore from "app/stores/NodeStore";
import Badge from "react-bootstrap/Badge";
import FormControl from "react-bootstrap/FormControl";
import InputGroup from "react-bootstrap/InputGroup";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import Popover from "react-bootstrap/Popover";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";

interface Props {
    visualizerStore?: VisualizerStore;
    nodeStore?: NodeStore;
}

@inject("visualizerStore")
@inject("nodeStore")
@observer
export class Visualizer extends React.Component<Props, any> {

    componentDidMount(): void {
        this.props.visualizerStore.start();
    }

    componentWillUnmount(): void {
        this.props.visualizerStore.stop();
        this.props.nodeStore.registerHandlers();
    }

    updateVerticesLimit = (e) => {
        this.props.visualizerStore.updateVerticesLimit(e.target.value);
    }

    pauseResumeVisualizer = (e) => {
        this.props.visualizerStore.pauseResume();
    }

    updateSearch = (e) => {
        this.props.visualizerStore.updateSearch(e.target.value);
    }

    searchAndHighlight = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.visualizerStore.searchAndHighlight();
    }

    toggleBackgroundDataCollection = () => {
        if (this.props.nodeStore.collecting) {
            this.props.nodeStore.unregisterHandlers();
            return;
        }
        this.props.nodeStore.registerHandlers();
    }

    render() {
        let {
            vertices, solid_count, selected,
            selected_approvers_count, selected_approvees_count,
            verticesLimit, tips_count, paused, search
        } = this.props.visualizerStore;
        let {last_mps_metric, collecting} = this.props.nodeStore;

        return (
            <Container>
                <h3>Visualizer</h3>
                <Row className={"mb-1"}>
                    <Col xs={5}>
                        <InputGroup className="mb-1" size="sm">
                            <InputGroup.Prepend>
                                <InputGroup.Text id="vertices-limit">Vertices Limit</InputGroup.Text>
                            </InputGroup.Prepend>
                            <FormControl
                                placeholder="limit"
                                type="number" value={verticesLimit.toString()} onChange={this.updateVerticesLimit}
                                aria-label="vertices-limit"
                                aria-describedby="vertices-limit"
                            />
                        </InputGroup>
                        <InputGroup className="mb-1" size="sm">
                            <InputGroup.Prepend>
                                <InputGroup.Text id="vertices-limit">
                                    Search Vertex
                                </InputGroup.Text>
                            </InputGroup.Prepend>
                            <FormControl
                                placeholder="search"
                                type="text" value={search} onChange={this.updateSearch}
                                aria-label="vertices-search" onKeyUp={this.searchAndHighlight}
                                aria-describedby="vertices-search"
                            />
                        </InputGroup>
                        <InputGroup className="mb-1" size="sm">
                            <OverlayTrigger
                                trigger={['hover', 'focus']} placement="right" overlay={
                                <Popover id="popover-basic">
                                    <Popover.Content>
                                        Ensures that only data needed for the visualizer is collected.
                                    </Popover.Content>
                                </Popover>}
                            >
                                <Button variant="outline-secondary" onClick={this.toggleBackgroundDataCollection}
                                        size="sm">
                                    {collecting ? "Stop Background Data Collection" : "Collect Background data"}
                                </Button>
                            </OverlayTrigger>
                            <br/>
                        </InputGroup>
                        <InputGroup className="mb-1" size="sm">
                            <OverlayTrigger
                                trigger={['hover', 'focus']} placement="right" overlay={
                                <Popover id="popover-basic">
                                    <Popover.Content>
                                        Pauses/resumes rendering the graph.
                                    </Popover.Content>
                                </Popover>}
                            >
                                <Button onClick={this.pauseResumeVisualizer} size="sm" variant="outline-secondary">
                                    {paused ? "Resume Rendering" : "Pause Rendering"}
                                </Button>
                            </OverlayTrigger>
                        </InputGroup>
                    </Col>
                    <Col xs={{span: 5, offset: 2}}>
                        <p>
                            <Badge pill style={{background: "#6c71c4", color: "white"}}>
                                Solid
                            </Badge>
                            {' '}
                            <Badge pill style={{background: "#2aa198", color: "white"}}>
                                Unsolid
                            </Badge>
                            {' '}
                            <Badge pill style={{background: "#cb4b16", color: "white"}}>
                                Tip
                            </Badge>
                            {' '}
                            <Badge pill style={{background: "#b58900", color: "white"}}>
                                Unknown
                            </Badge>
                            <br/>
                            Vertices: {vertices.size}, Tips: {tips_count},
                            Solid/Unsolid: {solid_count}/{vertices.size - solid_count},{' '}
                            MPS: {last_mps_metric.mps}
                            <br/>
                            Selected: {selected ?
                            <Link to={`/explorer/message/${selected.id}`}>
                                {selected.id.substr(0, 10)}
                            </Link>
                            : "-"}
                            <br/>
                            Approvers/Approvees: {selected ?
                            <span>{selected_approvers_count}/{selected_approvees_count}</span>
                            : '-/-'}
                            <br/>
                            Trunk/Branch:{' '}
                            {
                                selected && selected.trunk_id && selected.branch_id ?
                                    <span>
                                        <Link to={` / explorer / message /${selected.trunk_id}`}>
                                            {selected.trunk_id.substr(0, 10)}
                                        </Link>
                                        /
                                        <Link to={` / explorer / message /${selected.branch_id}`}>
                                            {selected.branch_id.substr(0, 10)}
                                        </Link>
                                    </span>
                                    : "-"}
                        </p>
                    </Col>
                </Row>
                <div className={"visualizer"} style={{
                    zIndex: -1, position: "absolute",
                    top: 0, left: 0,
                    width: "100%",
                    height: "100%",
                    background: "#ededed"
                }} id={"visualizer"}/>
            </Container>
        );
    }
}
