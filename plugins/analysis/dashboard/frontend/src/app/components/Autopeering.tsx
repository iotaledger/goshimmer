import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import {inject, observer} from "mobx-react";
import AutopeeringStore, {shortenedIDCharCount} from "app/stores/AutopeeringStore";
import {Col} from "react-bootstrap";
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
                <Row style={{paddingBottom: 10}}>
                    <Col style={{display: 'flex',  justifyContent:'center', alignItems:'center'}}>
                        <Badge pill style={{background: "#cb4b16", color: "white"}}>
                            Selected Node
                        </Badge>
                    </Col>
                </Row>
                <Row style={{paddingBottom: 20}}>
                    <Col style={{display: 'flex',  justifyContent:'center', alignItems:'center'}}>
                        <Button style={{fontSize: 14, backgroundColor: "#cb4b16"}} variant="danger">
                            {this.props.autopeeringStore.selectedNode.slice(0,shortenedIDCharCount)}
                        </Button>
                    </Col>
                </Row>
                <Row style={{paddingBottom: 10}}>
                    <Col style={{display: 'flex',  justifyContent:'center', alignItems:'center'}}>
                        <Badge pill style={{background: "#1c8d7f", color: "white"}}>
                            Incoming Neighbors ({this.props.autopeeringStore.selectedNodeInNeighbors.size.toString()})
                        </Badge>
                    </Col>
                    <Col style={{display: 'flex',  justifyContent:'center', alignItems:'center'}}>
                        <Badge pill style={{background: "#336db5", color: "white"}}>
                            Outgoing Neighbors ({this.props.autopeeringStore.selectedNodeOutNeighbors.size.toString()})
                        </Badge>
                    </Col>
                </Row>
                <Row style={{paddingBottom: 20}}>
                    <Col>
                        <ul style={{marginLeft: 25, listStylePosition: 'inside'}}>
                            {this.props.autopeeringStore.inNeighborList}
                        </ul>
                    </Col>
                    <Col>
                        <ul style={{marginLeft: 25, listStylePosition: 'inside'}}>
                            {this.props.autopeeringStore.outNeighborList}
                        </ul>
                    </Col>
                </Row>
                <Row>
                    <Col style={{display: 'flex',  justifyContent:'center', alignItems:'center'}}>
                        <Button style={{fontSize: 11}} variant="info" onClick={this.props.autopeeringStore.clearNodeSelection}>
                            Clear Selection
                        </Button>
                    </Col>
                </Row>
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
                    <Col xs={6}>
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
                        </InputGroup>
                        <div style={{height: 220, overflow: 'auto'}}>
                            {nodeListView}
                        </div>
                    </Col>
                    <Col xs={6} style={{height: 352.5,  overflow:'auto'}}>
                        <NodeView></NodeView>
                    </Col>
                </Row>
                    <div className={"visualizer"} style={{
                        zIndex: -1,
                        top: 0, left: 0,
                        width: "100%",
                        height: "600px",
                        background: "#202126",
                        borderRadius: 20,
                    }} id={"visualizer"}/>
            </Container>
        );
    }
}
