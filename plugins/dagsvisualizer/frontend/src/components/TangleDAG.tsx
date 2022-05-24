import * as React from 'react';
import Container from 'react-bootstrap/Container';
import { inject, observer } from 'mobx-react';
import { MdKeyboardArrowDown, MdKeyboardArrowUp } from 'react-icons/md';
import { Collapse } from 'react-bootstrap';
import TangleStore from 'stores/TangleStore';
import { MessageInfo } from 'components/MessageInfo';
import 'styles/style.css';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Button from 'react-bootstrap/Button';
import Popover from 'react-bootstrap/Popover';
import OverlayTrigger from 'react-bootstrap/OverlayTrigger';
import InputGroup from 'react-bootstrap/InputGroup';
import FormControl from 'react-bootstrap/FormControl';
import GlobalStore from '../stores/GlobalStore';
import { TangleLegend } from './Legend';

interface Props {
    tangleStore?: TangleStore;
    globalStore?: GlobalStore;
}

@inject('tangleStore')
@inject('globalStore')
@observer
export default class TangleDAG extends React.Component<Props, any> {
    constructor(props) {
        super(props);
        this.state = { open: true };
    }

    componentDidMount() {
        this.props.tangleStore.start();
    }

    componentWillUnmount() {
        this.props.tangleStore.stop();
    }

    pauseResumeVisualizer = () => {
        this.props.tangleStore.pauseResume();
    };

    updateVerticesLimit = (e) => {
        this.props.tangleStore.updateVerticesLimit(e.target.value);
    };

    updateSearch = (e) => {
        this.props.tangleStore.updateSearch(e.target.value);
    };

    searchAndSelect = (e: any) => {
        if (e.key !== 'Enter') return;
        this.props.tangleStore.searchAndSelect();
    };

    centerGraph = () => {
        this.props.tangleStore.centerEntireGraph();
    };

    syncWithMsg = () => {
        this.props.globalStore.syncWithMsg();
    };

    render() {
        const { paused, maxTangleVertices, search } = this.props.tangleStore;

        return (
            <Container>
                <div
                    onClick={() =>
                        this.setState((prevState) => ({
                            open: !prevState.open
                        }))
                    }
                >
                    <h2>
                        Tangle DAG
                        {this.state.open ? (
                            <MdKeyboardArrowUp />
                        ) : (
                            <MdKeyboardArrowDown />
                        )}
                    </h2>
                </div>
                <Collapse in={this.state.open}>
                    <div className={'panel'}>
                        <Row xs={5}>
                            <Col
                                className="align-self-end"
                                style={{
                                    display: 'flex',
                                    justifyContent: 'space-evenly'
                                }}
                            >
                                <InputGroup className="mb-1">
                                    <OverlayTrigger
                                        trigger={['hover', 'focus']}
                                        placement="right"
                                        overlay={
                                            <Popover id="popover-basic">
                                                <Popover.Body>
                                                    Pauses/resumes rendering the
                                                    graph.
                                                </Popover.Body>
                                            </Popover>
                                        }
                                    >
                                        <Button
                                            className={'button'}
                                            onClick={this.pauseResumeVisualizer}
                                            variant="outline-secondary"
                                        >
                                            {paused
                                                ? 'Resume Rendering'
                                                : 'Pause Rendering'}
                                        </Button>
                                    </OverlayTrigger>
                                </InputGroup>
                                <InputGroup className="mb-1">
                                    <Button
                                        className={'button'}
                                        onClick={this.centerGraph}
                                        variant="outline-secondary"
                                    >
                                        Center Graph
                                    </Button>
                                </InputGroup>
                                <InputGroup className="mb-1">
                                    <Button
                                        className={'button'}
                                        onClick={this.syncWithMsg}
                                        variant="outline-secondary"
                                    >
                                        Sync with msg
                                    </Button>
                                </InputGroup>
                            </Col>
                            <Col>
                                <InputGroup className="mb-1">
                                    <InputGroup.Text id="vertices-limit">
                                        Vertices Limit
                                    </InputGroup.Text>
                                    <FormControl
                                        placeholder="limit"
                                        value={maxTangleVertices.toString()}
                                        onChange={this.updateVerticesLimit}
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
                                        type="text"
                                        value={search}
                                        onChange={this.updateSearch}
                                        aria-label="vertices-search"
                                        onKeyUp={this.searchAndSelect}
                                        aria-describedby="vertices-search"
                                    />
                                </InputGroup>
                            </Col>
                        </Row>
                        <div className="graphFrame">
                            <MessageInfo />
                            <div id="tangleVisualizer" />
                        </div>
                        <TangleLegend />
                    </div>
                </Collapse>
                <br />
            </Container>
        );
    }
}
