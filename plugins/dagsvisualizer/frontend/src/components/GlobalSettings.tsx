import * as React from 'react';
import Container from 'react-bootstrap/Container';
import {inject, observer} from "mobx-react";
import { MdKeyboardArrowDown, MdKeyboardArrowUp } from 'react-icons/md';
import { Collapse } from 'react-bootstrap';
import InputGroup from "react-bootstrap/InputGroup";
import FormControl from "react-bootstrap/FormControl";
import GlobalStore from "stores/GlobalStore";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import Datetime from 'react-datetime';
import {isMoment} from 'moment';
import "react-datetime/css/react-datetime.css";

interface Props {
    globalStore?: GlobalStore;
}

@inject("globalStore")
@observer
export class GlobalSettings extends React.Component<Props, any> {
    constructor(props) {
        super(props);
        this.state = {isIdle: true, open: true, explorerAddress:""};  
    }

    updateFrom = (date) => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchStartingTime(date);
        }        
    }

    updateTo = (date) => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchEndingTime(date);
        }   
    }

    searchVerticesInLedger = () => {
        this.setState({isIdle: false});
        this.props.globalStore.searchAndDrawResults();
    }

    clearSearch = () => {
        this.setState({isIdle: true});
        this.props.globalStore.clearSearchAndResume();
    }

    updateFormInput = (e) => {
        this.setState({ [e.target.name]: e.target.value });
    };

    setExplorerAddress = (e) => {
        if (e.key === 'Enter') {
            this.props.globalStore.updateExplorerAddress(this.state.explorerAddress);
            this.setState({explorerAddress: ""})
        }
    };

    render () {
        return (
            <Container>
                <div onClick={() => this.setState(prevState => ({open: !prevState.open}))}>
                        <h2 >
                            Global Functions 
                            { this.state.open ? <MdKeyboardArrowUp /> : <MdKeyboardArrowDown /> }
                        </h2>
                </div>
                <Collapse in={this.state.open}>
                    <div>
                        <div>
                            <h5>Search Vertex Within Time Intervals</h5>
                            <Row xs={5}>
                                <Col>
                                    From: <Datetime onChange={this.updateFrom} />
                                </Col>
                                <Col>
                                    To: <Datetime onChange={this.updateTo} />
                                </Col>
                                <Col className="align-self-end" style={{display: "flex", justifyContent: "space-evenly"}}>
                                    <Button onClick={this.searchVerticesInLedger} variant="outline-secondary">
                                        Search
                                    </Button>
                                    <Button disabled={this.state.isIdle} onClick={this.clearSearch} variant="outline-secondary">
                                        Clear and Resume
                                    </Button>
                                </Col>        
                            </Row>
                        </div>
                        <div>
                            <h5 style={{marginTop: "10px"}}>Set explorer URL</h5>
                            <p> default is the local explorer: <i>localhost:8081</i> </p>
                            <Row xs={5}>
                                <Col>
                                    <InputGroup className="mb-3">
                                        <FormControl
                                            placeholder="explorer URL"
                                            aria-label="explorer URL"
                                            name="explorerAddress"
                                            aria-describedby="basic-addon1"
                                            value={this.state.explorerAddress} onChange={this.updateFormInput}
                                            onKeyUp={this.setExplorerAddress}
                                        />
                                    </InputGroup>
                                </Col>
                            </Row>
                        </div>
                    </div>
                </Collapse>
                <br></br>
            </Container>
        );
    }
}