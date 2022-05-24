import * as React from 'react';
import Container from 'react-bootstrap/Container';
import { inject, observer } from 'mobx-react';
import { MdKeyboardArrowDown, MdKeyboardArrowUp } from 'react-icons/md';
import { Collapse } from 'react-bootstrap';
import BranchStore from 'stores/BranchStore';
import { BranchInfo } from 'components/BranchInfo';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Button from 'react-bootstrap/Button';
import Popover from 'react-bootstrap/Popover';
import OverlayTrigger from 'react-bootstrap/OverlayTrigger';
import InputGroup from 'react-bootstrap/InputGroup';
import FormControl from 'react-bootstrap/FormControl';
import 'styles/style.css';
import GlobalStore from '../stores/GlobalStore';
import { BranchLegend } from './Legend';

interface Props {
    branchStore?: BranchStore;
    globalStore?: GlobalStore;
}

@inject('branchStore')
@inject('globalStore')
@observer
export default class BranchDAG extends React.Component<Props, any> {
    constructor(props) {
        super(props);
        this.state = { isIdle: true, open: true };
    }

    componentDidMount() {
        this.props.branchStore.start();
    }

    componentWillUnmount() {
        this.props.branchStore.stop();
    }

    pauseResumeVisualizer = () => {
        this.props.branchStore.pauseResume();
    };

    updateVerticesLimit = (e) => {
        this.props.branchStore.updateVerticesLimit(e.target.value);
    };

    updateSearch = (e) => {
        this.props.branchStore.updateSearch(e.target.value);
    };

    searchAndHighlight = (e: any) => {
        if (e.key !== 'Enter') return;
        this.props.branchStore.searchAndHighlight();
    };

    centerGraph = () => {
        this.props.branchStore.centerEntireGraph();
    };

    syncWithBranch = () => {
        this.props.globalStore.syncWithBranch();
    };

    render() {
        const { paused, maxBranchVertices, search } = this.props.branchStore;

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
                        Branch DAG
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
                                        onClick={this.syncWithBranch}
                                        variant="outline-secondary"
                                    >
                                        Sync with branch
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
                                        value={maxBranchVertices.toString()}
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
                            <BranchInfo />
                            <div id="branchVisualizer" />
                        </div>
                        <BranchLegend />
                    </div>
                </Collapse>
                <br />
            </Container>
        );
    }
}
