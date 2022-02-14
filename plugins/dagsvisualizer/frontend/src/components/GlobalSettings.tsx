import * as React from 'react';
import Container from 'react-bootstrap/Container';
import { inject, observer } from 'mobx-react';
import { MdKeyboardArrowDown, MdKeyboardArrowUp } from 'react-icons/md';
import { Collapse } from 'react-bootstrap';
import InputGroup from 'react-bootstrap/InputGroup';
import FormControl from 'react-bootstrap/FormControl';
import GlobalStore from 'stores/GlobalStore';
import Row from 'react-bootstrap/Row';
import Col from 'react-bootstrap/Col';
import Button from 'react-bootstrap/Button';
import moment, { isMoment } from 'moment';
import 'react-datetime/css/react-datetime.css';
import { Picker, TimePickerButtons } from './timeButtons';

interface Props {
    globalStore?: GlobalStore;
}

@inject('globalStore')
@observer
export default class GlobalSettings extends React.Component<Props, any> {
    constructor(props) {
        super(props);
        this.state = {
            isIdle: true,
            searching: false,
            open: true,
            searchOpen: true,
            dashboardUrlOpen: true,
            syncOpen: true,
            explorerAddress: ''
        };
    }

    updateFrom = (date) => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchStartingTime(date);
        }
    };

    updateTo = (date) => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchEndingTime(date);
        }
    };

    searchVerticesInLedger = () => {
        this.setState({ searching: true, isIdle: false });
        this.props.globalStore
            .searchAndDrawResults()
            .then(() => this.setState({ searching: false }));
    };

    clearSearch = () => {
        this.setState({ isIdle: true });
        this.props.globalStore.clearSearchAndResume();
    };

    updateFormInput = (e) => {
        this.setState({ [e.target.name]: e.target.value });
    };

    setExplorerAddress = (e) => {
        if (e.key === 'Enter') {
            this.props.globalStore.updateExplorerAddress(
                this.state.explorerAddress
            );
            this.setState({ explorerAddress: '' });
        }
    };

    syncWithMsg = () => {
        this.props.globalStore.syncWithMsg();
    };

    syncWithTx = () => {
        this.props.globalStore.syncWithTx();
    };

    syncWithBranch = () => {
        this.props.globalStore.syncWithBranch();
    };

    clearSync = () => {
        this.props.globalStore.clearSync();
    };

    onOpenStartPicker = () => {
        this.props.globalStore.updateStartManualPicker(false);
    };

    onOpenEndPicker = () => {
        this.props.globalStore.updateEndManualPicker(false);
    };

    renderSearchResults = () => {
        this.props.globalStore.renderSearchResults();
    };

    render() {
        const pickerStartValue = this.props.globalStore.manualPicker
            ? moment.unix(this.props.globalStore.searchStartingTime)
            : undefined;
        const pickerEndValue = this.props.globalStore.manualPicker
            ? moment.unix(this.props.globalStore.searchEndingTime)
            : undefined;
        const globalStore = this.props.globalStore;
        const { searchResponse, previewResponseSize } = this.props.globalStore;
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
                        Global Functions
                        {this.state.open ? (
                            <MdKeyboardArrowUp />
                        ) : (
                            <MdKeyboardArrowDown />
                        )}
                    </h2>
                </div>
                <Collapse in={this.state.open}>
                    <div>
                        <div className={'panel'}>
                            <div
                                onClick={() =>
                                    this.setState((prevState) => ({
                                        searchOpen: !prevState.searchOpen
                                    }))
                                }
                            >
                                <h5>
                                    Search Vertex Within Time Intervals{' '}
                                    {this.state.searchOpen ? (
                                        <MdKeyboardArrowUp />
                                    ) : (
                                        <MdKeyboardArrowDown />
                                    )}
                                </h5>
                            </div>
                            <Collapse in={this.state.searchOpen}>
                                <Row md={4}>
                                    <Col>
                                        From:{' '}
                                        <Picker
                                            isStartTime={true}
                                            globalStore={globalStore}
                                            dateTime={pickerStartValue}
                                            onOpenFunc={this.onOpenStartPicker}
                                        />
                                        <TimePickerButtons isStartTime={true} />
                                    </Col>
                                    <Col>
                                        To:{' '}
                                        <Picker
                                            isStartTime={false}
                                            globalStore={globalStore}
                                            dateTime={pickerEndValue}
                                            onOpenFunc={this.onOpenEndPicker}
                                        />
                                        <TimePickerButtons
                                            isStartTime={false}
                                        />
                                    </Col>
                                    <Col className="align-self-end">
                                        <Button
                                            className={'button'}
                                            onClick={
                                                this.searchVerticesInLedger
                                            }
                                            variant="outline-secondary"
                                        >
                                            {this.state.searching ? (
                                                <div>
                                                    <span className="spinner-border spinner-border-sm text-secondary" />{' '}
                                                    Searching...
                                                </div>
                                            ) : (
                                                'Search'
                                            )}
                                        </Button>
                                        <Button
                                            className={'button'}
                                            onClick={this.renderSearchResults}
                                            variant="outline-secondary"
                                        >
                                            Render
                                        </Button>
                                        <Button
                                            className={'button'}
                                            disabled={this.state.isIdle}
                                            onClick={this.clearSearch}
                                            variant="outline-secondary"
                                        >
                                            Clear and Resume
                                        </Button>
                                    </Col>

                                    <Col
                                        className="align-self-end"
                                        style={{
                                            display: 'flex',
                                            color: 'red'
                                        }}
                                    >
                                        <div>
                                            <p className={'response-info'}>
                                                {previewResponseSize}
                                            </p>
                                            <p>{searchResponse}</p>
                                        </div>
                                    </Col>
                                </Row>
                            </Collapse>
                        </div>
                        <div className={'panel'}>
                            <div
                                onClick={() =>
                                    this.setState((prevState) => ({
                                        dashboardUrlOpen:
                                            !prevState.dashboardUrlOpen
                                    }))
                                }
                            >
                                <h5 style={{ marginTop: '10px' }}>
                                    Set explorer URL{' '}
                                    {this.state.dashboardUrlOpen ? (
                                        <MdKeyboardArrowUp />
                                    ) : (
                                        <MdKeyboardArrowDown />
                                    )}
                                </h5>
                                <p>
                                    {' '}
                                    Default is the local explorer:{' '}
                                    <i>http://localhost:8081</i>{' '}
                                </p>
                            </div>
                            <Collapse in={this.state.dashboardUrlOpen}>
                                <Row xs={5}>
                                    <Col>
                                        <InputGroup className="mb-3">
                                            <FormControl
                                                placeholder="explorer URL"
                                                aria-label="explorer URL"
                                                name="explorerAddress"
                                                aria-describedby="basic-addon1"
                                                value={
                                                    this.state.explorerAddress
                                                }
                                                onChange={this.updateFormInput}
                                                onKeyUp={
                                                    this.setExplorerAddress
                                                }
                                            />
                                        </InputGroup>
                                    </Col>
                                </Row>
                            </Collapse>
                        </div>
                        <div className={'panel'}>
                            <div
                                onClick={() =>
                                    this.setState((prevState) => ({
                                        syncOpen: !prevState.syncOpen
                                    }))
                                }
                            >
                                <h5>
                                    Select and center vertex across DAGs{' '}
                                    {this.state.syncOpen ? (
                                        <MdKeyboardArrowUp />
                                    ) : (
                                        <MdKeyboardArrowDown />
                                    )}
                                </h5>
                                <p>
                                    {' '}
                                    Select a message/transaction/branch and
                                    click the corresponding button to sync.{' '}
                                </p>
                            </div>
                            <Collapse in={this.state.syncOpen}>
                                <Row>
                                    <Col xs="auto">
                                        <Button
                                            className={'button'}
                                            onClick={this.syncWithMsg}
                                            variant="outline-secondary"
                                        >
                                            Sync with message
                                        </Button>
                                    </Col>
                                    <Col xs="auto">
                                        <Button
                                            className={'button'}
                                            onClick={this.syncWithTx}
                                            variant="outline-secondary"
                                        >
                                            Sync with transaction
                                        </Button>
                                    </Col>
                                    <Col xs="auto">
                                        <Button
                                            className={'button'}
                                            onClick={this.syncWithBranch}
                                            variant="outline-secondary"
                                        >
                                            Sync with branch
                                        </Button>
                                    </Col>
                                    <Col xs="auto">
                                        <Button
                                            className={'button'}
                                            onClick={this.clearSync}
                                            variant="outline-secondary"
                                        >
                                            Clear
                                        </Button>
                                    </Col>
                                </Row>
                            </Collapse>
                        </div>
                    </div>
                </Collapse>
                <br />
            </Container>
        );
    }
}
