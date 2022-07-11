import * as React from 'react';
import Container from 'react-bootstrap/Container';
import {inject, observer} from 'mobx-react';
import {MdKeyboardArrowDown, MdKeyboardArrowUp} from 'react-icons/md';
import {Collapse} from 'react-bootstrap';
import ConflictStore from 'stores/ConflictStore';
import {ConflictInfo} from 'components/ConflictInfo';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Button from 'react-bootstrap/Button';
import Popover from 'react-bootstrap/Popover';
import OverlayTrigger from 'react-bootstrap/OverlayTrigger';
import InputGroup from 'react-bootstrap/InputGroup';
import FormControl from 'react-bootstrap/FormControl';
import 'styles/style.css';
import GlobalStore from '../stores/GlobalStore';
import {ConflictLegend} from './Legend';

interface Props {
    conflictStore?: ConflictStore;
    globalStore?: GlobalStore;
}

@inject('conflictStore')
@inject('globalStore')
@observer
export default class ConflictDAG extends React.Component<Props, any> {
    constructor(props) {
        super(props);
        this.state = { isIdle: true, open: true };
    }

    componentDidMount() {
        this.props.conflictStore.start();
    }

    componentWillUnmount() {
        this.props.conflictStore.stop();
    }

    pauseResumeVisualizer = () => {
        this.props.conflictStore.pauseResume();
    };

    updateVerticesLimit = (e) => {
        this.props.conflictStore.updateVerticesLimit(e.target.value);
    };

    updateSearch = (e) => {
        this.props.conflictStore.updateSearch(e.target.value);
    };

    searchAndHighlight = (e: any) => {
        if (e.key !== 'Enter') return;
        this.props.conflictStore.searchAndHighlight();
    };

    centerGraph = () => {
        this.props.conflictStore.centerEntireGraph();
    };

    syncWithConflict = () => {
        this.props.globalStore.syncWithConflict();
    };

    render() {
        const { paused, maxConflictVertices, search } = this.props.conflictStore;

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
                        Conflict DAG
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
                                        onClick={this.syncWithConflict}
                                        variant="outline-secondary"
                                    >
                                        Sync with conflict
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
                                        value={maxConflictVertices.toString()}
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
                                        onKeyUp={this.searchAndHighlight}
                                        aria-describedby="vertices-search"
                                    />
                                </InputGroup>
                            </Col>
                        </Row>
                        <div className="graphFrame">
                            <ConflictInfo />
                            <div id="conflictVisualizer" />
                        </div>
                        <ConflictLegend />
                    </div>
                </Collapse>
                <br />
            </Container>
        );
    }
}
