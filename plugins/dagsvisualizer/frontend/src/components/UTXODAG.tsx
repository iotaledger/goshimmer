import * as React from 'react';
import Container from 'react-bootstrap/Container';
import { inject, observer } from 'mobx-react';
import { MdKeyboardArrowDown, MdKeyboardArrowUp } from 'react-icons/md';
import { Collapse } from 'react-bootstrap';
import { TransactionInfo } from 'components/TransactionInfo';
import UTXOStore from 'stores/UTXOStore';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Button from 'react-bootstrap/Button';
import Popover from 'react-bootstrap/Popover';
import OverlayTrigger from 'react-bootstrap/OverlayTrigger';
import InputGroup from 'react-bootstrap/InputGroup';
import FormControl from 'react-bootstrap/FormControl';
import GlobalStore from '../stores/GlobalStore';
import { UTXOLegend } from './Legend';

interface Props {
    utxoStore?: UTXOStore;
    globalStore?: GlobalStore;
}

@inject('utxoStore')
@inject('globalStore')
@observer
export default class UTXODAG extends React.Component<Props, any> {
    constructor(props) {
        super(props);
        this.state = { isIdle: true, open: true };
    }

    componentDidMount() {
        this.props.utxoStore.start();
    }

    componentWillUnmount() {
        this.props.utxoStore.stop();
    }

    pauseResumeVisualizer = () => {
        this.props.utxoStore.pauseResume();
    };

    updateVerticesLimit = (e) => {
        this.props.utxoStore.updateVerticesLimit(e.target.value);
    };

    updateSearch = (e) => {
        this.props.utxoStore.updateSearch(e.target.value);
    };

    searchAndSelect = (e: any) => {
        if (e.key !== 'Enter') return;
        this.props.utxoStore.searchAndSelect();
    };

    centerGraph = () => {
        this.props.utxoStore.centerEntireGraph();
    };

    syncWithTx = () => {
        this.props.globalStore.syncWithTx();
    };

    render() {
        const { paused, maxUTXOVertices, search } = this.props.utxoStore;

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
                        UTXO DAG
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
                                        onClick={this.syncWithTx}
                                        variant="outline-secondary"
                                    >
                                        Sync with tx
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
                                        value={maxUTXOVertices.toString()}
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
                            <TransactionInfo />
                            <div id="utxoVisualizer" />
                        </div>
                        <UTXOLegend />
                    </div>
                </Collapse>
                <br />
            </Container>
        );
    }
}
