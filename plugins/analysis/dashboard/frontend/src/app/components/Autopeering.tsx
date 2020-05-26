import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import {inject, observer} from "mobx-react";
import AutopeeringStore from "app/stores/AutopeeringStore";
import {Col} from "react-bootstrap";
import ListGroup from "react-bootstrap/ListGroup";
import Badge from "react-bootstrap/Badge";
import FormControl from "react-bootstrap/FormControl";
import InputGroup from "react-bootstrap/InputGroup";
import Button from "react-bootstrap/Button";


interface Props {
    autopeeringStore?: AutopeeringStore
}

@inject("autopeeringStore")
@observer
export class NodeView extends React.Component<Props, any> {
    render() {
        if (this.props.autopeeringStore.selectedNode == null) {
            return (
                <div>
                    Click on a node in the list for details.
                </div>
            )
        };
        return (
            <div>
                <Badge pill style={{background: "#cb4b16", color: "white"}}>
                    Selected Node
                </Badge>
                {this.props.autopeeringStore.selectedNode}
                <br/>
                <Badge pill style={{background: "#1c8d7f", color: "white"}}>
                    Incoming Neighbors ({this.props.autopeeringStore.selectedNodeInNeighbors.size.toString()})
                </Badge>
                <ul>
                    {this.props.autopeeringStore.inNeighborList}
                </ul>
                <Badge pill style={{background: "#336db5", color: "white"}}>
                    Outgoing Neighbors ({this.props.autopeeringStore.selectedNodeOutNeighbors.size.toString()})
                </Badge>
                <ul>
                    {this.props.autopeeringStore.outNeighborList}
                </ul>
            </div>
        );
    }
}

@inject("autopeeringStore")
@observer
export class Autopeering extends React.Component<Props, any> {

    componentDidMount(): void {
        this.props.autopeeringStore.start();
    }

    componentWillUnmount(): void {
        this.props.autopeeringStore.stop();
    }

    updateSearch = (e) => {
        this.props.autopeeringStore.updateSearch(e.target.value);
    }

    render() {
        let {nodeListView, search} = this.props.autopeeringStore
        return (
            <Container>

                <Row className={"mb-1"}>
                    <Col xs={6} style={{paddingBottom: 30}}>
                        <Row className={"mb-1"}>
                            <h3>Autopeering Visualizer</h3>
                        </Row>
                        <Row className={"mb-1"}>
                            <Col >
                                <Badge pill style={{background: "#6c71c4", color: "white"}}>
                                    Nodes online: {this.props.autopeeringStore.nodes.size.toString()}
                                </Badge>
                                <Badge pill style={{background: "#b58900", color: "white"}}>
                                    Average number of neighbors: {
                                    (2*this.props.autopeeringStore.connections.size / this.props.autopeeringStore.nodes.size).toPrecision(2).toString()
                                }
                                </Badge>
                            </Col>
                        </Row>

                        Online nodes
                        <InputGroup className="mb-1" size="sm">
                            <InputGroup.Prepend>
                                <InputGroup.Text id="search">
                                    Search Node
                                </InputGroup.Text>
                            </InputGroup.Prepend>
                            <FormControl
                                placeholder="search"
                                type="text" value={search} onChange={this.updateSearch}
                                aria-label="node-search" onKeyUp={this.updateSearch}
                                aria-describedby="node-search"
                            />
                            <Button style={{fontSize: 14}} variant="secondary" onClick={this.props.autopeeringStore.clearSelection}>
                                Clear Selection
                            </Button>
                        </InputGroup>

                        <ListGroup style={{maxHeight: 180, overflow: 'auto'}}>
                            {nodeListView}
                        </ListGroup>
                    </Col>
                    <Col xs={6}>
                        <NodeView></NodeView>
                    </Col>
                </Row>
                    <div className={"visualizer"} style={{
                        zIndex: -1, position: "static",
                        top: 0, left: 0,
                        width: "100%",
                        height: "600px",
                        background: "#202126",
                    }} id={"visualizer"}/>


            </Container>
        );
    }
}
