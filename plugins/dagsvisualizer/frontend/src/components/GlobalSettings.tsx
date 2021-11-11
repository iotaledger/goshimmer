import * as React from 'react';
import Container from 'react-bootstrap/Container';
import {inject, observer} from "mobx-react";
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
        this.state = {isIdle: true};  
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

    render () {
        return (
            <Container>
                <h2> Global Functions </h2>
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
                <br></br>
            </Container>
        );
    }
}